package stories

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/decide"
	"github.com/alternayte/briefing/internal/embed"
	"github.com/alternayte/briefing/internal/ingest"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

// decisionStub answers every triage with worth and category taken from the
// item title, written as "worth=0.9 category=security".
func decisionStub(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			State string `json:"state"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var worth float64
		var category string
		title := strings.SplitN(strings.TrimPrefix(req.State, "Title: "), "\n", 2)[0]
		_, _ = fmt.Sscanf(title, "worth=%g category=%s", &worth, &category)
		fmt.Fprintf(w, `{"model":"cloudflare/clef","answers":{
			"worth":{"type":"noul","noul":%g},"sensational":{"type":"noul","noul":0.1},
			"category":{"type":"choice","choice":%q,"confidence":0.9},
			"significance":{"type":"score","score":2.6},"learning":{"type":"score","score":2.1}},
			"usage":{"input_tokens":600,"output_tokens":0,"cost":0.00002}}`, worth, category)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func insertItem(t *testing.T, ctx context.Context, url, title string, status ingest.ItemStatus) int64 {
	t.Helper()
	var id int64
	err := drel.MustFromContext(ctx).QueryRow(ctx,
		`INSERT INTO items (canonical_url, title, snippet, status) VALUES ($1, $2, 'snippet', $3) RETURNING id`,
		url, title, status).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTriageKeepsWorthyItemsOutsideOther(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	srv := decisionStub(t)
	cfg := platform.Config{DecideAPI: platform.DecideAPIDecisions, DecideModel: "cloudflare/clef"}
	tu := platform.DefaultTuning()
	tr := NewTriager(e, decide.New(platform.NewOpenRouter(srv.URL, "k", srv.Client(), e, time.Now), cfg, tu), tu, time.Now)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		keep := insertItem(t, ctx, "https://triage.test/keep", "worth=0.9 category=security", ingest.ItemPassed)
		weak := insertItem(t, ctx, "https://triage.test/weak", "worth=0.2 category=security", ingest.ItemPassed)
		other := insertItem(t, ctx, "https://triage.test/other", "worth=0.9 category=other", ingest.ItemPassed)
		filtered := insertItem(t, ctx, "https://triage.test/filtered", "worth=0.9 category=security", ingest.ItemFiltered)

		kept, err := tr.Triage(ctx, []int64{keep, weak, other, filtered})
		if err != nil {
			t.Fatal(err)
		}
		if len(kept) != 1 || kept[0] != keep {
			t.Errorf("kept = %v", kept)
		}
		tx := drel.MustFromContext(ctx)
		state := func(id int64) (status, category string) {
			_ = tx.QueryRow(ctx, `SELECT status, COALESCE(triage->>'category', '') FROM items WHERE id = $1`, id).Scan(&status, &category)
			return
		}
		for id, want := range map[int64]string{keep: "kept", weak: "rejected", other: "rejected", filtered: "filtered"} {
			if s, _ := state(id); s != want {
				t.Errorf("item %d: %s, want %s", id, s, want)
			}
		}
		if _, c := state(keep); c != "security" {
			t.Errorf("triage not stored: %q", c)
		}
		var calls int
		_ = tx.QueryRow(ctx, `SELECT COALESCE(sum(calls), 0) FROM llm_usage WHERE stage = 'triage' AND day = current_date`).Scan(&calls)
		if calls < 3 {
			t.Errorf("llm_usage has %d triage calls, want at least 3", calls)
		}
	})
}

func TestEmbedStoresVectorAndModel(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	data, err := os.ReadFile("../embed/testdata/embeddings.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
	defer srv.Close()
	or := platform.NewOpenRouter(srv.URL, "k", srv.Client(), nil, time.Now)
	em := NewItemEmbedder(e, embed.New(or, platform.Config{EmbedModel: "openai/text-embedding-3-small", EmbedDim: 1536}))
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		a := insertItem(t, ctx, "https://embed.test/a", "A", ingest.ItemKept)
		b := insertItem(t, ctx, "https://embed.test/b", "B", ingest.ItemKept)
		rejected := insertItem(t, ctx, "https://embed.test/r", "R", ingest.ItemRejected)
		done, err := em.Embed(ctx, []int64{a, b, rejected})
		if err != nil {
			t.Fatal(err)
		}
		if len(done) != 2 {
			t.Fatalf("embedded %v", done)
		}
		var dims int
		var model string
		if err := drel.MustFromContext(ctx).QueryRow(ctx, `SELECT vector_dims(embedding), embed_model FROM items WHERE id = $1`, a).Scan(&dims, &model); err != nil {
			t.Fatal(err)
		}
		if dims != 1536 || model != "openai/text-embedding-3-small" {
			t.Errorf("stored %d dims, model %q", dims, model)
		}
		again, err := em.Embed(ctx, []int64{a, b})
		if err != nil || len(again) != 0 {
			t.Errorf("re-embed: %v %v", again, err)
		}
	})
}
