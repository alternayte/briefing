package stories

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/decide"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

// sameEventStub answers same_event with the value after "same=" in the new
// item's title.
func sameEventStub(t *testing.T, asked *int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*asked++
		var req struct {
			State string `json:"state"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var p float64
		if i := strings.Index(req.State, "same="); i >= 0 {
			_, _ = fmt.Sscanf(req.State[i:], "same=%g", &p)
		}
		fmt.Fprintf(w, `{"answers":{"same_event":{"type":"noul","noul":%g}},"usage":{"input_tokens":10,"output_tokens":0,"cost":0}}`, p)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func keptItem(t *testing.T, ctx context.Context, url, title string, vec []float32, significance float64, category string) int64 {
	t.Helper()
	v, _ := platform.NewVector(vec).Value()
	triage := fmt.Sprintf(`{"worth":0.9,"sensational":0.1,"category":%q,"significance":%g,"learning":1.5}`, category, significance)
	var id int64
	err := drel.MustFromContext(ctx).QueryRow(ctx, `INSERT INTO items (canonical_url, title, snippet, status, triage, embedding, embed_model)
		VALUES ($1, $2, '', 'kept', $3::jsonb, $4::vector, 'test') RETURNING id`, url, title, triage, v).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestClusteringJoinsTheSameEventAndCountsPublishers(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	var asked int
	srv := sameEventStub(t, &asked)
	tu := platform.DefaultTuning()
	now := time.Date(2031, 3, 1, 12, 0, 0, 0, time.UTC)
	d := decide.New(platform.NewOpenRouter(srv.URL, "k", srv.Client(), nil, time.Now), platform.Config{DecideAPI: platform.DecideAPIDecisions}, tu)
	c := NewClusterer(e, d, tu, func() time.Time { return now })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		// Stories from other tests in this database must not attract these items.
		if _, err := drel.MustFromContext(ctx).Exec(ctx, `UPDATE stories SET status = 'closed'`); err != nil {
			t.Fatal(err)
		}
		a := keptItem(t, ctx, "https://www.alpha.example/launch", "Probe lands", []float32{1, 0, 0}, 3.1, "space")
		b := keptItem(t, ctx, "https://news.beta.example/probe", "Probe touches down", []float32{0.99, 0.05, 0}, 2.0, "physics")
		other := keptItem(t, ctx, "https://gamma.example/chip", "New chip", []float32{0, 1, 0}, 2.0, "hardware_chips")
		bandYes := keptItem(t, ctx, "https://delta.example/p", "Probe lands, same=0.9", []float32{0.8, 0.1, 0.59}, 1.0, "space")
		bandNo := keptItem(t, ctx, "https://epsilon.example/q", "Other probe, same=0.2", []float32{0.9, 0, -0.43}, 1.0, "space")

		touched, err := c.Cluster(ctx, []int64{a, b, other, bandYes, bandNo}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(touched) != 5 {
			t.Fatalf("touched %v", touched)
		}
		storyOf := map[int64]int64{}
		for i, id := range []int64{a, b, other, bandYes, bandNo} {
			storyOf[id] = touched[i]
		}
		if storyOf[a] != storyOf[b] || storyOf[a] != storyOf[bandYes] {
			t.Errorf("a, b and the confirmed band item must share a story: %v", storyOf)
		}
		if storyOf[other] == storyOf[a] || storyOf[bandNo] == storyOf[a] {
			t.Errorf("different events share a story: %v", storyOf)
		}
		if asked != 2 {
			t.Errorf("asked same_event %d times, want 2 (only band items)", asked)
		}
		var publishers int
		var category string
		var significance float64
		if err := drel.MustFromContext(ctx).QueryRow(ctx, `SELECT publisher_count, category, significance FROM stories WHERE id = $1`, storyOf[a]).
			Scan(&publishers, &category, &significance); err != nil {
			t.Fatal(err)
		}
		if publishers != 3 || category != "space" || significance != 3.1 {
			t.Errorf("story = %d publishers, %s, %v", publishers, category, significance)
		}

		again, err := c.Cluster(ctx, []int64{a, b}, nil)
		if err != nil || len(again) != 0 {
			t.Errorf("re-cluster placed items again: %v %v", again, err)
		}
	})
}

func TestCloseStaleClosesQuietStories(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	tu := platform.DefaultTuning()
	now := time.Date(2031, 3, 10, 12, 0, 0, 0, time.UTC)
	c := NewClusterer(e, nil, tu, func() time.Time { return now })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		var quiet, busy int64
		_ = tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category) VALUES ($1, $1, 'space') RETURNING id`, now.Add(-73*time.Hour)).Scan(&quiet)
		_ = tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category) VALUES ($1, $1, 'space') RETURNING id`, now.Add(-time.Hour)).Scan(&busy)
		if _, err := c.CloseStale(ctx); err != nil {
			t.Fatal(err)
		}
		var s1, s2 string
		_ = tx.QueryRow(ctx, `SELECT status FROM stories WHERE id = $1`, quiet).Scan(&s1)
		_ = tx.QueryRow(ctx, `SELECT status FROM stories WHERE id = $1`, busy).Scan(&s2)
		if s1 != "closed" || s2 != "open" {
			t.Errorf("quiet=%s busy=%s", s1, s2)
		}
	})
}
