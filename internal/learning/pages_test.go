package learning

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

func TestReviewAndLearnedPages(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	now := time.Date(2031, 12, 1, 9, 0, 0, 0, time.UTC)
	tu := platform.DefaultTuning()
	w := NewWeb(e, tu, func() time.Time { return now })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		user := pgtest.User(t, ctx, "pages@example.com")
		profile, _ := accounts.NewProfileStore(e, tu).Ensure(ctx, user, "P", i18n.EN)
		var story, concept, item int64
		_ = tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category, revision) VALUES ($1, $1, 'space', 1) RETURNING id`, now).Scan(&story)
		_, _ = tx.Exec(ctx, `INSERT INTO summaries (story_id, revision, headline, summary, why_it_matters, what_is_new, claims, confidence, model, prompt_version, publisher_count, text_items)
			VALUES ($1, 1, 'Probe lands', 's', 'w', 'n', '[]', 'medium', 'm', 'v', 1, 1)`, story)
		_ = tx.QueryRow(ctx, `INSERT INTO concepts (slug, name, definition) VALUES ('pages-regolith', 'Regolith', 'Loose rock on a surface.') RETURNING id`).Scan(&concept)
		_, _ = tx.Exec(ctx, `INSERT INTO story_concepts (story_id, concept_id) VALUES ($1, $2)`, story, concept)
		_ = tx.QueryRow(ctx, `INSERT INTO review_items (story_id, revision, question, choices, answer_index, explanation) VALUES ($1, 1, 'What is regolith?', '["Ice","Loose rock","Gas","Metal"]', 1, 'Regolith is loose rock.') RETURNING id`, story).Scan(&item)
		_, _ = tx.Exec(ctx, `INSERT INTO review_schedules (user_id, review_item_id, due_at, interval_days) VALUES ($1, $2, $3, 1)`, user, item, now.Add(-time.Hour))
		_, _ = tx.Exec(ctx, `INSERT INTO learning_events (user_id, story_id, kind, payload, created_at) VALUES ($1, $2, 'opened', '{}', $3), ($1, $2, 'useful', '{}', $3)`, user, story, now.Add(-time.Hour))

		rctx := i18n.WithLocale(accounts.WithProfile(context.WithValue(ctx, webKey{}, w), profile), i18n.EN)
		rec := httptest.NewRecorder()
		ReviewPage.ServeHTTP(rec, httptest.NewRequest("GET", "/review", nil).WithContext(rctx))
		for _, want := range []string{"What is regolith?", "Loose rock", "Probe lands"} {
			if !strings.Contains(rec.Body.String(), want) {
				t.Errorf("review page lacks %q", want)
			}
		}
		if _, err := AnswerReview(ctx, e, user, item, 1, now, tu.Review); err != nil {
			t.Fatal(err)
		}
		rec = httptest.NewRecorder()
		ReviewPage.ServeHTTP(rec, httptest.NewRequest("GET", "/review?answered="+strconv.FormatInt(item, 10)+"&choice=1", nil).WithContext(rctx))
		en := i18n.For(i18n.EN)
		for _, want := range []string{en.ReviewCorrect, "Regolith is loose rock.", "3 days", en.ReviewNone} {
			if !strings.Contains(rec.Body.String(), want) {
				t.Errorf("answered page lacks %q", want)
			}
		}

		rec = httptest.NewRecorder()
		LearnedPageHandler.ServeHTTP(rec, httptest.NewRequest("GET", "/learned", nil).WithContext(rctx))
		body := rec.Body.String()
		for _, want := range []string{"Regolith", "Loose rock on a surface.", en.ConceptFamiliar, "1 story read", "Probe lands"} {
			if !strings.Contains(body, want) {
				t.Errorf("learned page lacks %q", want)
			}
		}
	})
}
