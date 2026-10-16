package write

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/embed"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

var sourceID = regexp.MustCompile(`<source id="(\d+)"`)

// modelStub replays the recorded writer answer with the fixture's item ids
// replaced by the ids of the request, and embeds each concept along its own
// axis. With broken set, the writer answer has no claims.
type modelStub struct {
	srv    *httptest.Server
	writes atomic.Int32
	broken bool
}

func newModelStub(t *testing.T) *modelStub {
	t.Helper()
	raw, err := os.ReadFile("testdata/chat-write.json")
	if err != nil {
		t.Fatal(err)
	}
	var recorded struct {
		Choices []struct {
			Message struct{ Content string } `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &recorded); err != nil {
		t.Fatal(err)
	}
	content := recorded.Choices[0].Message.Content
	m := &modelStub{}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct{ Content string } `json:"messages"`
			Input    []string                   `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if r.URL.Path == "/api/v1/embeddings" {
			type item struct {
				Index     int       `json:"index"`
				Embedding []float32 `json:"embedding"`
			}
			var data []item
			for i, text := range req.Input {
				v := make([]float32, 8)
				v[len(text)%8] = 1
				data = append(data, item{Index: i, Embedding: v})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "usage": map[string]any{"prompt_tokens": 5, "cost": 0.000001}})
			return
		}
		m.writes.Add(1)
		ids := sourceID.FindAllStringSubmatch(req.Messages[1].Content, -1)
		out := content
		if len(ids) > 0 {
			second := ids[0][1]
			if len(ids) > 1 {
				second = ids[1][1]
			}
			out = strings.NewReplacer(`"101"`, `"`+ids[0][1]+`"`, `"102"`, `"`+second+`"`).Replace(out)
		}
		if m.broken {
			out = `{"headline":"H","summary":"S","why_it_matters":"W","what_is_new":"N","claims":[],"concepts":[],"review":[]}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"finish_reason": "stop", "message": map[string]string{"content": out}}},
			"usage":   map[string]any{"prompt_tokens": 1000, "completion_tokens": 900, "cost": 0.0006},
		})
	}))
	t.Cleanup(m.srv.Close)
	return m
}

func newTestWriter(e *drel.Engine, m *modelStub, capUSD float64, now time.Time) *Writer {
	or := platform.NewOpenRouter(m.srv.URL, "k", m.srv.Client(), e, func() time.Time { return now })
	cfg := platform.Config{WriteModel: "anthropic/claude-haiku-5.5", EmbedModel: "test-embed", EmbedDim: 8, DailySpendUSD: capUSD}
	return NewWriter(e, or, embed.New(or, cfg), cfg, platform.DefaultTuning(), func() time.Time { return now })
}

// storyWith creates a story with one item per text length, on its own
// publisher, and returns the story id.
func storyWith(t *testing.T, ctx context.Context, publishers int, textLens ...int) int64 {
	t.Helper()
	tx := drel.MustFromContext(ctx)
	var story int64
	if err := tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category, publisher_count) VALUES (now(), now(), 'space', $1) RETURNING id`, publishers).Scan(&story); err != nil {
		t.Fatal(err)
	}
	for i, n := range textLens {
		var item int64
		url := fmt.Sprintf("https://pub%d-%d.example/a", story, i)
		if err := tx.QueryRow(ctx, `INSERT INTO items (canonical_url, title, snippet, status, text, text_len, text_source)
			VALUES ($1, 'Probe lands on a comet', '', 'kept', $2, $3, 'extract') RETURNING id`, url, strings.Repeat("x", n), n).Scan(&item); err != nil {
			t.Fatal(err)
		}
		_, _ = tx.Exec(ctx, `INSERT INTO story_items (story_id, item_id) VALUES ($1, $2)`, story, item)
	}
	return story
}

func TestWriteStoresTheEntry(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	m := newModelStub(t)
	now := time.Date(2031, 4, 1, 9, 0, 0, 0, time.UTC)
	w := newTestWriter(e, m, 1, now)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		story := storyWith(t, ctx, 2, 4000, 2000)
		ok, err := w.Write(ctx, story)
		if err != nil || !ok {
			t.Fatalf("written=%v err=%v", ok, err)
		}
		tx := drel.MustFromContext(ctx)
		var revision, reviews, concepts int
		var conf, claims string
		_ = tx.QueryRow(ctx, `SELECT revision FROM stories WHERE id = $1`, story).Scan(&revision)
		_ = tx.QueryRow(ctx, `SELECT confidence, claims::text FROM summaries WHERE story_id = $1 AND revision = 1`, story).Scan(&conf, &claims)
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM review_items WHERE story_id = $1 AND revision = 1`, story).Scan(&reviews)
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM story_concepts WHERE story_id = $1`, story).Scan(&concepts)
		if revision != 1 || conf != "high" || reviews != 2 || concepts != 3 || strings.Contains(claims, `"101"`) {
			t.Errorf("revision=%d confidence=%s reviews=%d concepts=%d claims=%s", revision, conf, reviews, concepts, claims)
		}

		// Unchanged coverage writes nothing new.
		again, err := w.Write(ctx, story)
		if err != nil || again || m.writes.Load() != 1 {
			t.Errorf("rewrite: %v %v, %d calls", again, err, m.writes.Load())
		}
		// Doubled coverage writes revision 2 and reuses the concepts.
		_, _ = tx.Exec(ctx, `UPDATE stories SET publisher_count = 4 WHERE id = $1`, story)
		if ok, err := w.Write(ctx, story); err != nil || !ok {
			t.Fatalf("revision 2: %v %v", ok, err)
		}
		var conceptRows int
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM story_concepts WHERE story_id = $1`, story).Scan(&conceptRows)
		if conceptRows != 3 {
			t.Errorf("revision 2 linked %d concepts, want the same 3", conceptRows)
		}
	})
}

func TestWriteSkipsBelowTheEvidenceFloor(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	m := newModelStub(t)
	w := newTestWriter(e, m, 1, time.Date(2031, 4, 2, 9, 0, 0, 0, time.UTC))
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		story := storyWith(t, ctx, 3, 1499, 800)
		if ok, err := w.Write(ctx, story); ok || err != nil || m.writes.Load() != 0 {
			t.Errorf("written=%v err=%v calls=%d", ok, err, m.writes.Load())
		}
	})
}

func TestWriteStopsAtTheSpendCap(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	m := newModelStub(t)
	now := time.Date(2031, 4, 3, 9, 0, 0, 0, time.UTC)
	w := newTestWriter(e, m, 0.001, now)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		_, _ = drel.MustFromContext(ctx).Exec(ctx, `INSERT INTO llm_usage (day, provider, stage, model, calls, cost_usd) VALUES ('2031-04-03', 'openrouter', 'triage', 'm', 1, 0.0011)`)
		story := storyWith(t, ctx, 1, 4000)
		if ok, err := w.Write(ctx, story); ok || err != nil || m.writes.Load() != 0 {
			t.Errorf("written=%v err=%v calls=%d", ok, err, m.writes.Load())
		}
	})
}

func TestWriteGivesUpAfterTwoInvalidAnswers(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	m := newModelStub(t)
	m.broken = true
	w := newTestWriter(e, m, 1, time.Date(2031, 4, 4, 9, 0, 0, 0, time.UTC))
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		story := storyWith(t, ctx, 1, 4000)
		if ok, err := w.Write(ctx, story); ok || err != nil || m.writes.Load() != 2 {
			t.Errorf("written=%v err=%v calls=%d", ok, err, m.writes.Load())
		}
		// A second run the same day makes one more call, then the daily
		// attempt limit of 3 stops it.
		_, _ = w.Write(ctx, story)
		_, _ = w.Write(ctx, story)
		if m.writes.Load() != 3 {
			t.Errorf("%d calls, want 3 in one day", m.writes.Load())
		}
		var n int
		_ = drel.MustFromContext(ctx).QueryRow(ctx, `SELECT count(*) FROM summaries WHERE story_id = $1`, story).Scan(&n)
		if n != 0 {
			t.Error("an invalid entry was stored")
		}
	})
}
