package ranking

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/decide"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

// relevanceStub reads "rel=N exc=P" from the story headline.
func relevanceStub(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct{ State string }
		_ = json.NewDecoder(r.Body).Decode(&req)
		var rel, exc float64
		if i := strings.Index(req.State, "rel="); i >= 0 {
			_, _ = fmt.Sscanf(req.State[i:], "rel=%g exc=%g", &rel, &exc)
		}
		fmt.Fprintf(w, `{"answers":{"relevance":{"type":"score","score":%g},"excluded":{"type":"noul","noul":%g}},"usage":{"input_tokens":1,"output_tokens":0,"cost":0}}`, rel, exc)
	}))
	t.Cleanup(srv.Close)
	return srv
}

type fixture struct {
	t   *testing.T
	ctx context.Context
	tx  *drel.Tx
	now time.Time
}

func (f fixture) reader(email string, budget int, interest []float32, text string) uuid.UUID {
	id := pgtest.User(f.t, f.ctx, email)
	if _, err := f.tx.Exec(f.ctx, `INSERT INTO profiles (id, display_name, locale, daily_budget) VALUES ($1, 'r', 'en', $2)`, id, budget); err != nil {
		f.t.Fatal(err)
	}
	v, _ := platform.NewVector(interest).Value()
	if _, err := f.tx.Exec(f.ctx, `INSERT INTO interests (user_id, text, embedding, embed_model) VALUES ($1, $2, $3::vector, 'test')`, id, text, v); err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f fixture) story(headline string, centroid []float32, age time.Duration, significance float64) int64 {
	v, _ := platform.NewVector(centroid).Value()
	var id int64
	err := f.tx.QueryRow(f.ctx, `INSERT INTO stories (opened_at, last_item_at, category, centroid, significance, learning, publisher_count)
		VALUES ($1, $1, 'x', $2::vector, $3, 1.5, 2) RETURNING id`, f.now.Add(-age), v, significance).Scan(&id)
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.tx.Exec(f.ctx, `INSERT INTO summaries (story_id, revision, headline, summary, why_it_matters, what_is_new, claims, confidence, model, prompt_version, publisher_count, text_items)
		VALUES ($1, 1, $2, 's', 'w', 'n', '[]', 'medium', 'm', 'v', 2, 1)`, id, headline); err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f fixture) feed(user uuid.UUID, state string) []int64 {
	rows, err := f.tx.Query(f.ctx, `SELECT story_id FROM feed_entries WHERE user_id = $1 AND state = $2 ORDER BY score DESC`, user, state)
	if err != nil {
		f.t.Fatal(err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids
}

func TestRankFillsEachReadersFeedFromTheirInterests(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	srv := relevanceStub(t)
	tu := platform.DefaultTuning()
	now := time.Date(2031, 7, 1, 12, 0, 0, 0, time.UTC)
	d := decide.New(platform.NewOpenRouter(srv.URL, "k", srv.Client(), e, func() time.Time { return now }), platform.Config{DecideAPI: platform.DecideAPIDecisions}, tu)
	r := NewRanker(e, d, tu, func() time.Time { return now })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		f := fixture{t: t, ctx: ctx, tx: drel.MustFromContext(ctx), now: now}
		// Stories from other tests must not enter these feeds.
		_, _ = f.tx.Exec(ctx, `UPDATE stories SET last_item_at = '2000-01-01'`)
		rustReader := f.reader("rust@example.com", 2, []float32{1, 0, 0}, "Rust")
		bioReader := f.reader("bio@example.com", 2, []float32{0, 1, 0}, "biology")

		core := f.story("Rust 2.0 ships rel=3 exc=0", []float32{1, 0, 0}, time.Hour, 3)
		near := f.story("Rust compiler faster rel=2 exc=0", []float32{0.99, 0.14, 0}, 2*time.Hour, 2)
		bio := f.story("Cell atlas complete rel=3 exc=0", []float32{0, 1, 0}, time.Hour, 3)
		f.story("Old Rust news rel=3 exc=0", []float32{1, 0, 0}, 100*time.Hour, 3)

		res, err := r.Rank(ctx, rustReader)
		if err != nil {
			t.Fatal(err)
		}
		slices.Sort(res.Relevant)
		if want := []int64{core, near}; !slices.Equal(res.Relevant, want) || res.Delivered != 2 {
			t.Errorf("rust reader: relevant %v, delivered %d", res.Relevant, res.Delivered)
		}
		if got := f.feed(rustReader, "delivered"); len(got) != 2 || got[0] != core {
			t.Errorf("rust feed = %v, want core first", got)
		}

		res, _ = r.Rank(ctx, bioReader)
		if got := f.feed(bioReader, "delivered"); len(got) != 1 || got[0] != bio {
			t.Errorf("bio feed = %v", got)
		}

		// A later story waits under More once the budget is spent.
		late := f.story("Rust 2.1 rel=3 exc=0", []float32{1, 0, 0}, 0, 3)
		if _, err := r.Rank(ctx, rustReader); err != nil {
			t.Fatal(err)
		}
		if got := f.feed(rustReader, "candidate"); len(got) != 1 || got[0] != late {
			t.Errorf("waiting candidates = %v", got)
		}

		var writes int
		_ = f.tx.QueryRow(ctx, `SELECT COALESCE(sum(calls), 0) FROM llm_usage WHERE stage = 'write' AND day = '2031-07-01'`).Scan(&writes)
		if writes != 0 {
			t.Errorf("ranking made %d write calls", writes)
		}
	})
}

func TestKnewLowersSimilarStoriesForThatReaderOnly(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	srv := relevanceStub(t)
	tu := platform.DefaultTuning()
	now := time.Date(2031, 7, 2, 12, 0, 0, 0, time.UTC)
	d := decide.New(platform.NewOpenRouter(srv.URL, "k", srv.Client(), nil, time.Now), platform.Config{DecideAPI: platform.DecideAPIDecisions}, tu)
	r := NewRanker(e, d, tu, func() time.Time { return now })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		f := fixture{t: t, ctx: ctx, tx: drel.MustFromContext(ctx), now: now}
		_, _ = f.tx.Exec(ctx, `UPDATE stories SET last_item_at = '2000-01-01'`)
		a := f.reader("a@example.com", 5, []float32{1, 0, 0}, "Rust")
		b := f.reader("b@example.com", 5, []float32{1, 0, 0}, "Rust")
		known := f.story("Rust release rel=3 exc=0", []float32{1, 0, 0}, time.Hour, 3)
		similar := f.story("Rust release notes rel=3 exc=0", []float32{0.99, 0.1, 0}, time.Hour, 3)
		_, _ = r.Rank(ctx, a)
		_, _ = r.Rank(ctx, b)
		score := func(u uuid.UUID, s int64) float64 {
			var v float64
			_ = f.tx.QueryRow(ctx, `SELECT score FROM feed_entries WHERE user_id = $1 AND story_id = $2`, u, s).Scan(&v)
			return v
		}
		beforeA, beforeB := score(a, similar), score(b, similar)
		_, _ = f.tx.Exec(ctx, `INSERT INTO learning_events (user_id, story_id, kind, payload, created_at) VALUES ($1, $2, 'knew', '{}', $3)`, a, known, now)
		_, _ = r.Rank(ctx, a)
		_, _ = r.Rank(ctx, b)
		if score(a, similar) >= beforeA-0.2 {
			t.Errorf("reader a: %v → %v, want a clear drop", beforeA, score(a, similar))
		}
		if score(b, similar) != beforeB {
			t.Errorf("reader b changed: %v → %v", beforeB, score(b, similar))
		}
	})
}
