package feed

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"
	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

// pageRequest is a request from the Gx runtime of an open page.
func pageRequest(ctx context.Context, method, target, body string) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body)).WithContext(ctx)
	r.Header.Set("Datastar-Request", "true")
	r.Header.Set("Accept", "text/event-stream")
	if body != "" {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return r
}

// deliveredStory adds a story with a summary to the reader's feed of today.
func deliveredStory(t *testing.T, ctx context.Context, tx *drel.Tx, user any, now time.Time, headline string, deliveredAt time.Time) int64 {
	t.Helper()
	var story, item int64
	if err := tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category, publisher_count, revision) VALUES ($1, $1, 'space', 1, 1) RETURNING id`, now).Scan(&story); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `INSERT INTO items (canonical_url, title, snippet, status) VALUES ($1, 't', '', 'kept') RETURNING id`, fmt.Sprintf("https://news.test/%d", story)).Scan(&item); err != nil {
		t.Fatal(err)
	}
	_, _ = tx.Exec(ctx, `INSERT INTO story_items (story_id, item_id) VALUES ($1, $2)`, story, item)
	_, _ = tx.Exec(ctx, `INSERT INTO summaries (story_id, revision, headline, summary, why_it_matters, what_is_new, claims, confidence, model, prompt_version, publisher_count, text_items, created_at)
		VALUES ($1, 1, $2, 's', 'w', 'n', '[]', 'high', 'm', 'v', 1, 1, $3)`, story, headline, deliveredAt)
	if _, err := tx.Exec(ctx, `INSERT INTO feed_entries (user_id, story_id, relevance, excluded, score, reason, state, delivered_on, delivered_at, shown_revision)
		VALUES ($1, $2, 1, 0, 0.5, '', 'delivered', $3::date, $4, 1)`, user, story, now.Format(time.DateOnly), deliveredAt); err != nil {
		t.Fatal(err)
	}
	return story
}

// Feedback from the feed patches the card in place; a redirect would move
// the reader back to the top of the page.
func TestFeedbackInThePagePatchesTheCard(t *testing.T) {
	gx.SetAdapter(datastar.Adapter())
	e := pgtest.Engine(t)
	now := time.Date(2031, 8, 1, 12, 0, 0, 0, time.UTC)
	tu := platform.DefaultTuning()
	w := NewWeb(e, nil, nil, nil, accounts.NewProfileStore(e, tu), nil, tu, "https://briefing.test", func() time.Time { return now })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		user := pgtest.User(t, ctx, "inplace@example.com")
		profile, err := accounts.NewProfileStore(e, tu).Ensure(ctx, user, "In place", i18n.EN)
		if err != nil {
			t.Fatal(err)
		}
		story := deliveredStory(t, ctx, tx, user, now, "Comet probe lands", now.Add(-time.Hour))
		rctx := i18n.WithLocale(accounts.WithProfile(context.WithValue(ctx, webKey{}, w), profile), i18n.EN)
		form := url.Values{"kind": {"knew"}, "from": {""}}
		req := pageRequest(rctx, "POST", fmt.Sprintf("/s/%d/feedback", story), form.Encode())
		req.SetPathValue("StoryID", fmt.Sprint(story))
		rec := httptest.NewRecorder()
		Feedback.ServeHTTP(rec, req)
		body := rec.Body.String()
		if strings.Contains(body, "redirect") || strings.Contains(body, "window.location") {
			t.Errorf("feedback answered with a redirect: %s", body)
		}
		if !strings.Contains(body, fmt.Sprintf(`id="entry-%d"`, story)) || !strings.Contains(body, `aria-pressed="true"`) {
			t.Errorf("feedback did not patch the card with the chosen button: %d %s", rec.Code, body)
		}
	})
}

// The banner counts only what a reload would show: a newly delivered story.
// A rescore, feedback or an opened story touches an entry but shows nothing new.
func TestFreshCountsOnlyNewDeliveries(t *testing.T) {
	gx.SetAdapter(datastar.Adapter())
	e := pgtest.Engine(t)
	now := time.Date(2031, 8, 1, 12, 0, 0, 0, time.UTC)
	tu := platform.DefaultTuning()
	w := NewWeb(e, nil, nil, nil, accounts.NewProfileStore(e, tu), nil, tu, "https://briefing.test", func() time.Time { return now })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		user := pgtest.User(t, ctx, "fresh@example.com")
		profile, err := accounts.NewProfileStore(e, tu).Ensure(ctx, user, "Fresh", i18n.EN)
		if err != nil {
			t.Fatal(err)
		}
		loaded := now.Add(-10 * time.Minute)
		story := deliveredStory(t, ctx, tx, user, now, "Old news", loaded.Add(-time.Hour))
		_, _ = tx.Exec(ctx, `UPDATE feed_entries SET score = 0.9, updated_at = $2 WHERE story_id = $1`, story, now)
		rctx := i18n.WithLocale(accounts.WithProfile(context.WithValue(ctx, webKey{}, w), profile), i18n.EN)
		fresh := func() string {
			rec := httptest.NewRecorder()
			FreshAction.ServeHTTP(rec, pageRequest(rctx, "GET", fmt.Sprintf("/feed/fresh?since=%d", loaded.Unix()), ""))
			return rec.Body.String()
		}
		banner := i18n.For(i18n.EN).FeedNewStories
		if strings.Contains(fresh(), banner) {
			t.Error("banner after a rescore with no new story")
		}
		deliveredStory(t, ctx, tx, user, now, "New news", now.Add(-time.Minute))
		if !strings.Contains(fresh(), banner) {
			t.Error("no banner after a new delivery")
		}
	})
}
