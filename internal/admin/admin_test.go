package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

func TestNonAdminsGetNotFound(t *testing.T) {
	w := NewWeb(nil, platform.Config{}, time.Now)
	rec := httptest.NewRecorder()
	called := false
	w.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).
		ServeHTTP(rec, httptest.NewRequest("GET", "/admin", nil))
	if rec.Code != http.StatusNotFound || called {
		t.Errorf("status %d, handler called %v", rec.Code, called)
	}
}

func TestDashboardShowsSourcesJobsAndSpend(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	now := time.Date(2031, 5, 1, 10, 0, 0, 0, time.UTC)
	w := NewWeb(e, platform.Config{DailySpendUSD: 0.5}, func() time.Time { return now })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		_, _ = tx.Exec(ctx, `INSERT INTO sources (kind, url, title, tier, language, poll_interval_seconds, last_error)
			VALUES ('rss', 'https://admin.test/feed', 'Admin Test Feed', 1, 'en', 1800, 'GET https://admin.test/feed: status 503')`)
		_, _ = tx.Exec(ctx, `INSERT INTO llm_usage (day, provider, stage, model, calls, cost_usd) VALUES ('2031-05-01', 'openrouter', 'write', 'm', 12, 0.6)`)

		ctx = i18n.WithLocale(context.WithValue(ctx, webKey{}, w), i18n.FR)
		rec := httptest.NewRecorder()
		Page.ServeHTTP(rec, httptest.NewRequest("GET", "/admin", nil).WithContext(ctx))
		body := rec.Body.String()
		fr := i18n.For(i18n.FR)
		for _, want := range []string{"Admin Test Feed", "status 503", fr.AdminSpend, fr.AdminCapReached, fr.AdminDisable, "0,60 $"} {
			if !strings.Contains(body, want) {
				t.Errorf("page lacks %q", want)
			}
		}
	})
}
