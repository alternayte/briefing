package sources

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/platform/pgtest"
)

func TestDueSelectsSourcesWhoseIntervalPassed(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		// Other tests may leave sources in this database; disable them here.
		if _, err := tx.Exec(ctx, `UPDATE sources SET enabled = false`); err != nil {
			t.Fatal(err)
		}
		mk := func(url string, lastOK *time.Time, lastErr string, updated time.Time, enabled bool) int64 {
			var id int64
			err := tx.QueryRow(ctx, `INSERT INTO sources (kind, url, title, tier, language, enabled, poll_interval_seconds, last_ok_at, last_error, updated_at)
				VALUES ('rss', $1, 't', 1, 'en', $2, 1800, $3, $4, $5) RETURNING id`, url, enabled, lastOK, lastErr, updated).Scan(&id)
			if err != nil {
				t.Fatal(err)
			}
			return id
		}
		ago := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }
		never := mk("https://due.test/never", nil, "", now, true)
		overdue := mk("https://due.test/overdue", ago(31*time.Minute), "", *ago(31 * time.Minute), true)
		mk("https://due.test/fresh", ago(10*time.Minute), "", *ago(10 * time.Minute), true)
		mk("https://due.test/failed-recently", ago(2*time.Hour), "boom", *ago(5 * time.Minute), true)
		failedLongAgo := mk("https://due.test/failed-long-ago", ago(2*time.Hour), "boom", *ago(40 * time.Minute), true)
		mk("https://due.test/disabled", nil, "", now, false)

		got, err := Due(ctx, e, now)
		if err != nil {
			t.Fatal(err)
		}
		want := []int64{never, overdue, failedLongAgo}
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("due = %v, want %v", got, want)
		}
	})
}
