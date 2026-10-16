package accounts

import (
	"context"
	"testing"

	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

func TestEnsureProfileCreatesOnceWithFirstLocale(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	profiles := NewProfileStore(e, platform.DefaultTuning())
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		id := pgtest.User(t, ctx, "ann@example.com")

		p, err := profiles.Ensure(ctx, id, "Ann", i18n.FR)
		if err != nil {
			t.Fatal(err)
		}
		if p.UserLocale() != i18n.FR || p.DailyBudget != platform.DefaultTuning().Rank.DefaultDailyFeed {
			t.Errorf("new profile = %+v", p)
		}

		again, err := profiles.Ensure(ctx, id, "Other", i18n.EN)
		if err != nil {
			t.Fatal(err)
		}
		if again.UserLocale() != i18n.FR || again.DisplayName != "Ann" {
			t.Errorf("second Ensure changed the profile: %+v", again)
		}
	})
}

func TestSetLocalePersists(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	profiles := NewProfileStore(e, platform.DefaultTuning())
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		id := pgtest.User(t, ctx, "bob@example.com")
		if _, err := profiles.Ensure(ctx, id, "Bob", i18n.EN); err != nil {
			t.Fatal(err)
		}
		if err := profiles.SetLocale(ctx, id, i18n.FR); err != nil {
			t.Fatal(err)
		}
		p, err := profiles.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if p.UserLocale() != i18n.FR {
			t.Errorf("locale = %s", p.Locale)
		}
	})
}
