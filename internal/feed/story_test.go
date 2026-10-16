package feed

import (
	"context"
	"net/http/httptest"
	"strconv"
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

func TestStoryViewShowsTheEntryAndMarksItSeen(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	now := time.Date(2031, 8, 2, 12, 0, 0, 0, time.UTC)
	tu := platform.DefaultTuning()
	w := NewWeb(e, nil, accounts.NewProfileStore(e, tu), nil, tu, func() time.Time { return now })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		user := pgtest.User(t, ctx, "story@example.com")
		profile, _ := accounts.NewProfileStore(e, tu).Ensure(ctx, user, "S", i18n.EN)
		profile.OnboardedAt = &now
		var story, a, b, concept int64
		_ = tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category, publisher_count, revision) VALUES ($1, $1, 'space', 2, 1) RETURNING id`, now).Scan(&story)
		_ = tx.QueryRow(ctx, `INSERT INTO items (canonical_url, title, snippet, status, text, text_len) VALUES ('https://alpha.test/probe', 'Alpha title', '', 'kept', 'First paragraph.

Second paragraph.', 40) RETURNING id`).Scan(&a)
		_ = tx.QueryRow(ctx, `INSERT INTO items (canonical_url, title, snippet, status) VALUES ('https://beta.test/probe', 'Beta title', '', 'kept') RETURNING id`).Scan(&b)
		_, _ = tx.Exec(ctx, `INSERT INTO story_items (story_id, item_id) VALUES ($1, $2), ($1, $3)`, story, a, b)
		_, _ = tx.Exec(ctx, `INSERT INTO summaries (story_id, revision, headline, summary, why_it_matters, what_is_new, claims, confidence, model, prompt_version, publisher_count, text_items)
			VALUES ($1, 1, 'Probe lands', 'It landed.', 'It matters.', 'It is new.', $2::jsonb, 'high', 'm', 'v', 2, 1)`,
			story, `[{"text":"The probe landed on Tuesday.","item_ids":[`+itoa(a)+`,`+itoa(b)+`]}]`)
		_ = tx.QueryRow(ctx, `INSERT INTO concepts (slug, name, definition) VALUES ('soft-landing', 'Soft landing', 'A landing at low speed.') RETURNING id`).Scan(&concept)
		_, _ = tx.Exec(ctx, `INSERT INTO story_concepts (story_id, concept_id) VALUES ($1, $2)`, story, concept)
		_, _ = tx.Exec(ctx, `INSERT INTO feed_entries (user_id, story_id, relevance, excluded, score, state, delivered_on)
			VALUES ($1, $2, 3, 0, 0.8, 'delivered', '2031-08-02')`, user, story)

		rctx := i18n.WithLocale(accounts.WithProfile(context.WithValue(ctx, webKey{}, w), profile), i18n.EN)
		view := func() string {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/s/"+itoa(story), nil).WithContext(rctx)
			req.SetPathValue("ID", itoa(story))
			StoryPage.ServeHTTP(rec, req)
			return rec.Body.String()
		}
		body := view()
		view()
		en := i18n.For(i18n.EN)
		for _, want := range []string{"Probe lands", "It matters.", "The probe landed on Tuesday.", "[1]", "[2]", "Soft landing", en.ConceptNew, "Second paragraph.", en.StoryNoText} {
			if !strings.Contains(body, want) {
				t.Errorf("story view lacks %q", want)
			}
		}
		var opened int
		var state string
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM learning_events WHERE user_id = $1 AND story_id = $2 AND kind = 'opened'`, user, story).Scan(&opened)
		_ = tx.QueryRow(ctx, `SELECT state FROM feed_entries WHERE user_id = $1 AND story_id = $2`, user, story).Scan(&state)
		if opened != 1 || state != "seen" {
			t.Errorf("opened events %d, entry %s", opened, state)
		}

		rec := httptest.NewRecorder()
		HomePage.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil).WithContext(rctx))
		if !strings.Contains(rec.Body.String(), en.FeedCaughtUp) {
			t.Error("feed is not caught up after the only story was opened")
		}
	})
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
