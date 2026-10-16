package feed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

func TestSignedOutVisitorsGoToLogin(t *testing.T) {
	w := NewWeb(nil, nil, nil, nil, platform.DefaultTuning(), time.Now)
	rec := httptest.NewRecorder()
	w.Middleware(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Errorf("got %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestHomeShowsTodaysStoriesAndSendsNewReadersToOnboarding(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	now := time.Date(2031, 8, 1, 12, 0, 0, 0, time.UTC)
	tu := platform.DefaultTuning()
	w := NewWeb(e, nil, accounts.NewProfileStore(e, tu), nil, tu, func() time.Time { return now })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		user := pgtest.User(t, ctx, "home@example.com")
		profiles := accounts.NewProfileStore(e, tu)
		profile, err := profiles.Ensure(ctx, user, "Home", i18n.FR)
		if err != nil {
			t.Fatal(err)
		}
		get := func() *httptest.ResponseRecorder {
			rctx := i18n.WithLocale(accounts.WithProfile(context.WithValue(ctx, webKey{}, w), profile), i18n.FR)
			rec := httptest.NewRecorder()
			HomePage.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil).WithContext(rctx))
			return rec
		}
		if rec := get(); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/onboarding" {
			t.Fatalf("new reader: %d %q", rec.Code, rec.Header().Get("Location"))
		}

		onboarded := now
		profile.OnboardedAt = &onboarded
		var story, item int64
		_ = tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category, publisher_count, revision) VALUES ($1, $1, 'space', 2, 1) RETURNING id`, now).Scan(&story)
		_ = tx.QueryRow(ctx, `INSERT INTO items (canonical_url, title, snippet, status) VALUES ('https://www.example-news.test/probe', 't', '', 'kept') RETURNING id`).Scan(&item)
		_, _ = tx.Exec(ctx, `INSERT INTO story_items (story_id, item_id) VALUES ($1, $2)`, story, item)
		_, _ = tx.Exec(ctx, `INSERT INTO summaries (story_id, revision, headline, summary, why_it_matters, what_is_new, claims, confidence, model, prompt_version, publisher_count, text_items)
			VALUES ($1, 1, 'Probe lands on a comet', 'The probe touched down.', 'w', 'n', '[]', 'high', 'm', 'v', 2, 2)`, story)
		_, _ = tx.Exec(ctx, `INSERT INTO feed_entries (user_id, story_id, relevance, excluded, score, reason, state, delivered_on)
			VALUES ($1, $2, 3, 0, 0.8, 'space', 'delivered', '2031-08-01')`, user, story)

		rec := get()
		body := rec.Body.String()
		fr := i18n.For(i18n.FR)
		for _, want := range []string{"Probe lands on a comet", "example-news.test", fr.ConfidenceHigh, fr.FeedBecause} {
			if !strings.Contains(body, want) {
				t.Errorf("feed lacks %q", want)
			}
		}
	})
}
