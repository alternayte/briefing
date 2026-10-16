package sources

import (
	"context"
	"testing"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

func TestSeedIsRepeatableAndKeepsOperatorState(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	tu := platform.DefaultTuning()
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		first, err := ParseCatalog([]byte(`
- {kind: rss, title: Old title, tier: 2, url: "https://seed-test.example/feed"}
- {kind: bluesky, title: Someone, tier: 1, url: seed-test.example}
`), tu)
		if err != nil {
			t.Fatal(err)
		}
		if err := Seed(ctx, e, first); err != nil {
			t.Fatal(err)
		}
		// An operator turns a source off and it records a checkpoint.
		if _, err := drel.MustFromContext(ctx).Exec(ctx,
			`UPDATE sources SET enabled = false, etag = 'abc' WHERE url = 'https://seed-test.example/feed'`); err != nil {
			t.Fatal(err)
		}

		again, _ := ParseCatalog([]byte(`
- {kind: rss, title: New title, tier: 1, url: "https://seed-test.example/feed"}
- {kind: bluesky, title: Someone, tier: 1, url: seed-test.example}
`), tu)
		if err := Seed(ctx, e, again); err != nil {
			t.Fatal(err)
		}

		var n int
		var title, etag string
		var tier int
		var enabled bool
		tx := drel.MustFromContext(ctx)
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM sources WHERE url LIKE '%seed-test.example%'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT title, tier, enabled, etag FROM sources WHERE url = 'https://seed-test.example/feed'`).Scan(&title, &tier, &enabled, &etag); err != nil {
			t.Fatal(err)
		}
		if n != 2 || title != "New title" || tier != 1 || enabled || etag != "abc" {
			t.Errorf("after reseed: n=%d title=%q tier=%d enabled=%v etag=%q", n, title, tier, enabled, etag)
		}
	})
}
