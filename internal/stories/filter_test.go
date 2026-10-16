package stories

import (
	"context"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/ingest"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

func TestRulesExplainEachDrop(t *testing.T) {
	tu := platform.DefaultTuning()
	tu.Filter.BlockedDomains = []string{"spam.example"}
	rules, err := NewRules(tu.Filter)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }
	cases := []struct {
		item ingest.Item
		want string
	}{
		{ingest.Item{Title: "Rust 2.0 ships", CanonicalURL: "https://blog.rust-lang.org/x", PublishedAt: at(time.Hour)}, ""},
		{ingest.Item{Title: "No date given", CanonicalURL: "https://a.example/x"}, ""},
		{ingest.Item{Title: "  ", CanonicalURL: "https://a.example/x", PublishedAt: at(time.Hour)}, ReasonEmptyTitle},
		{ingest.Item{Title: "Old news", CanonicalURL: "https://a.example/x", PublishedAt: at(73 * time.Hour)}, ReasonTooOld},
		{ingest.Item{Title: "Fine title", CanonicalURL: "https://www.spam.example/x", PublishedAt: at(time.Hour)}, ReasonBlockedDomain},
		{ingest.Item{Title: "The best laptop deals this week", CanonicalURL: "https://a.example/x", PublishedAt: at(time.Hour)}, ReasonDenyPattern},
		{ingest.Item{Title: "Sponsored: try our cloud", CanonicalURL: "https://a.example/x", PublishedAt: at(time.Hour)}, ReasonDenyPattern},
		{ingest.Item{Title: "Microsoft and OpenAI rework their deal", CanonicalURL: "https://a.example/x", PublishedAt: at(time.Hour)}, ""},
	}
	for _, c := range cases {
		if got := rules.Check(&c.item, now); got != c.want {
			t.Errorf("%q: got %q, want %q", c.item.Title, got, c.want)
		}
	}
}

func TestFilterMovesNewItemsOnly(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	now := time.Now().UTC()
	f, err := NewFilter(e, platform.DefaultTuning(), func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		mk := func(url, title string, status ingest.ItemStatus) int64 {
			var id int64
			err := tx.QueryRow(ctx, `INSERT INTO items (canonical_url, title, snippet, published_at, status) VALUES ($1, $2, '', $3, $4) RETURNING id`,
				url, title, now.Add(-time.Hour), status).Scan(&id)
			if err != nil {
				t.Fatal(err)
			}
			return id
		}
		good := mk("https://filter.test/good", "A real finding", ingest.ItemNew)
		bad := mk("https://filter.test/bad", "", ingest.ItemNew)
		kept := mk("https://filter.test/kept", "", ingest.ItemKept)

		passed, err := f.Apply(ctx, []int64{good, bad, kept})
		if err != nil {
			t.Fatal(err)
		}
		if len(passed) != 1 || passed[0] != good {
			t.Errorf("passed = %v", passed)
		}
		status := func(id int64) (s, reason string) {
			_ = tx.QueryRow(ctx, `SELECT status, filter_reason FROM items WHERE id = $1`, id).Scan(&s, &reason)
			return
		}
		if s, _ := status(good); s != string(ingest.ItemPassed) {
			t.Errorf("good: %s", s)
		}
		if s, r := status(bad); s != string(ingest.ItemFiltered) || r != ReasonEmptyTitle {
			t.Errorf("bad: %s %s", s, r)
		}
		if s, _ := status(kept); s != string(ingest.ItemKept) {
			t.Errorf("kept item changed to %s", s)
		}
	})
}
