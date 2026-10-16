package stories

import (
	"context"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/ingest"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

func TestFindStalledListsItemsWaitingAtEachStep(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	now := time.Now().UTC()
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		if _, err := tx.Exec(ctx, `UPDATE items SET updated_at = now()`); err != nil {
			t.Fatal(err)
		}
		old := now.Add(-time.Hour)
		mk := func(url string, status ingest.ItemStatus, model string, at time.Time) int64 {
			var id int64
			if err := tx.QueryRow(ctx, `INSERT INTO items (canonical_url, title, snippet, status, embed_model, updated_at)
				VALUES ($1, 't', '', $2, $3, $4) RETURNING id`, url, status, model, at).Scan(&id); err != nil {
				t.Fatal(err)
			}
			return id
		}
		stuckNew := mk("https://stall.test/new", ingest.ItemNew, "", old)
		mk("https://stall.test/fresh", ingest.ItemNew, "", now)
		stuckPassed := mk("https://stall.test/passed", ingest.ItemPassed, "", old)
		stuckKept := mk("https://stall.test/kept", ingest.ItemKept, "", old)
		unplaced := mk("https://stall.test/unplaced", ingest.ItemKept, "m", old)
		placed := mk("https://stall.test/placed", ingest.ItemKept, "m", old)
		var story int64
		_ = tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category) VALUES (now(), now(), 'x') RETURNING id`).Scan(&story)
		_, _ = tx.Exec(ctx, `INSERT INTO story_items (story_id, item_id) VALUES ($1, $2)`, story, placed)

		s, err := FindStalled(ctx, e, now, 5*time.Minute, 100, 1.5)
		if err != nil {
			t.Fatal(err)
		}
		if len(s.New) != 1 || s.New[0] != stuckNew || len(s.Passed) != 1 || s.Passed[0] != stuckPassed ||
			len(s.Kept) != 1 || s.Kept[0] != stuckKept || len(s.Unplaced) != 1 || s.Unplaced[0] != unplaced {
			t.Errorf("stalled = %+v", s)
		}
	})
}

func TestShortlistKeepsNotableStories(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		mk := func(sig float64) int64 {
			var id int64
			_ = tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category, significance) VALUES (now(), now(), 'x', $1) RETURNING id`, sig).Scan(&id)
			return id
		}
		minor, notable, major := mk(1.2), mk(1.6), mk(3.4)
		got, err := Shortlist(ctx, e, []int64{minor, notable, major}, 1.5)
		if err != nil || len(got) != 2 || got[0] != notable || got[1] != major {
			t.Errorf("shortlist = %v (%v)", got, err)
		}
	})
}
