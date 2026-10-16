package translate

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

// frenchStub prefixes every string value with "FR " and keeps the shape;
// with dropClaim it loses one claim.
type frenchStub struct {
	srv       *httptest.Server
	calls     atomic.Int32
	dropClaim bool
}

func newFrenchStub(t *testing.T) *frenchStub {
	t.Helper()
	f := &frenchStub{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		var req struct{ Messages []platform.ChatMessage }
		_ = json.NewDecoder(r.Body).Decode(&req)
		var v any
		_ = json.Unmarshal([]byte(req.Messages[1].Content), &v)
		v = prefix(v)
		if m, ok := v.(map[string]any); ok && f.dropClaim {
			if cs, ok := m["claims"].([]any); ok && len(cs) > 0 {
				m["claims"] = cs[1:]
			}
		}
		out, _ := json.Marshal(v)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"finish_reason": "stop", "message": map[string]string{"content": string(out)}}},
			"usage":   map[string]any{"prompt_tokens": 500, "completion_tokens": 600, "cost": 0.0004},
		})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func prefix(v any) any {
	switch t := v.(type) {
	case string:
		return "FR " + t
	case []any:
		for i := range t {
			t[i] = prefix(t[i])
		}
	case map[string]any:
		for k := range t {
			if k != "id" {
				t[k] = prefix(t[k])
			}
		}
	}
	return v
}

func story(t *testing.T, ctx context.Context) int64 {
	t.Helper()
	tx := drel.MustFromContext(ctx)
	var id, concept int64
	_ = tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category, revision) VALUES (now(), now(), 'space', 1) RETURNING id`).Scan(&id)
	_, err := tx.Exec(ctx, `INSERT INTO summaries (story_id, revision, headline, summary, why_it_matters, what_is_new, claims, confidence, model, prompt_version, publisher_count, text_items)
		VALUES ($1, 1, 'Probe lands', 'It landed.', 'It matters.', 'It is new.', '[{"text":"Claim one.","item_ids":[]},{"text":"Claim two.","item_ids":[]}]', 'medium', 'm', 'v', 1, 1)`, id)
	if err != nil {
		t.Fatal(err)
	}
	_ = tx.QueryRow(ctx, `INSERT INTO concepts (slug, name, definition) VALUES ($1, 'Regolith', 'Loose rock.') RETURNING id`, "regolith-"+time.Now().Format("150405.000000")).Scan(&concept)
	_, _ = tx.Exec(ctx, `INSERT INTO story_concepts (story_id, concept_id) VALUES ($1, $2)`, id, concept)
	_, _ = tx.Exec(ctx, `INSERT INTO review_items (story_id, revision, question, choices, answer_index, explanation) VALUES ($1, 1, 'Where?', '["A","B","C","D"]', 0, 'Because.')`, id)
	return id
}

func TestStoryIsTranslatedOncePerRevisionAndLocale(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	stub := newFrenchStub(t)
	now := time.Date(2031, 10, 1, 9, 0, 0, 0, time.UTC)
	or := platform.NewOpenRouter(stub.srv.URL, "k", stub.srv.Client(), e, func() time.Time { return now })
	tr := New(e, or, platform.Config{TranslateModel: "m", DailySpendUSD: 1}, 5, func() time.Time { return now })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		id := story(t, ctx)
		for range 3 {
			if err := tr.TranslateStory(ctx, id, i18n.FR); err != nil {
				t.Fatal(err)
			}
		}
		if err := tr.TranslateStory(ctx, id, i18n.EN); err != nil {
			t.Fatal(err)
		}
		if n := stub.calls.Load(); n != 1 {
			t.Errorf("%d translation calls for one story and three French requests", n)
		}
		f, ok, err := Story(ctx, drel.MustFromContext(ctx), id, i18n.FR)
		if err != nil || !ok {
			t.Fatalf("cached: %v %v", ok, err)
		}
		if f.Headline != "FR Probe lands" || len(f.Claims) != 2 || f.Concepts[0].Name != "FR Regolith" || f.Review[0].Choices[3] != "FR D" {
			t.Errorf("translation = %+v", f)
		}
		if f.Concepts[0].ID == 0 || f.Review[0].ID == 0 {
			t.Error("ids were not kept")
		}
	})
}

func TestBrokenTranslationIsNotStored(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	stub := newFrenchStub(t)
	stub.dropClaim = true
	or := platform.NewOpenRouter(stub.srv.URL, "k", stub.srv.Client(), e, time.Now)
	tr := New(e, or, platform.Config{TranslateModel: "m", DailySpendUSD: 1}, 5, time.Now)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		id := story(t, ctx)
		if err := tr.TranslateStory(ctx, id, i18n.FR); err == nil {
			t.Fatal("a translation that lost a claim was accepted")
		}
		if _, ok, _ := Story(ctx, drel.MustFromContext(ctx), id, i18n.FR); ok {
			t.Error("a broken translation was stored")
		}
	})
}

func TestTranslationStopsAtTheCap(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	stub := newFrenchStub(t)
	now := time.Date(2031, 10, 2, 9, 0, 0, 0, time.UTC)
	or := platform.NewOpenRouter(stub.srv.URL, "k", stub.srv.Client(), e, func() time.Time { return now })
	tr := New(e, or, platform.Config{TranslateModel: "m", DailySpendUSD: 0.01}, 5, func() time.Time { return now })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		_, _ = drel.MustFromContext(ctx).Exec(ctx, `INSERT INTO llm_usage (day, provider, stage, model, calls, cost_usd) VALUES ('2031-10-02', 'openrouter', 'write', 'm', 1, 0.02)`)
		if err := tr.TranslateStory(ctx, story(t, ctx), i18n.FR); !errors.Is(err, ErrCapReached) || stub.calls.Load() != 0 {
			t.Errorf("err = %v, calls %d", err, stub.calls.Load())
		}
	})
}

func TestArticleTranslationIsLimitedAndCached(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	stub := newFrenchStub(t)
	now := time.Date(2031, 10, 3, 9, 0, 0, 0, time.UTC)
	or := platform.NewOpenRouter(stub.srv.URL, "k", stub.srv.Client(), e, func() time.Time { return now })
	tr := New(e, or, platform.Config{TranslateModel: "m", DailySpendUSD: 1}, 1, func() time.Time { return now })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		user := pgtest.User(t, ctx, "article@example.com")
		item := func(url string) int64 {
			var id int64
			_ = tx.QueryRow(ctx, `INSERT INTO items (canonical_url, title, snippet, text, text_len) VALUES ($1, 't', '', 'First.

Second.', 15) RETURNING id`, url).Scan(&id)
			return id
		}
		a, b := item("https://article.test/a"), item("https://article.test/b")
		paras, err := tr.TranslateArticle(ctx, user, a, i18n.FR)
		if err != nil || len(paras) != 2 || !strings.HasPrefix(paras[1], "FR ") {
			t.Fatalf("paragraphs %q (%v)", paras, err)
		}
		if _, err := tr.TranslateArticle(ctx, user, a, i18n.FR); err != nil || stub.calls.Load() != 1 {
			t.Errorf("cached article made %d calls (%v)", stub.calls.Load(), err)
		}
		if _, err := tr.TranslateArticle(ctx, user, b, i18n.FR); !errors.Is(err, ErrArticleLimit) {
			t.Errorf("second article of the day: %v", err)
		}
	})
}

func TestMissingListsShownStoriesWithoutTranslation(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	stub := newFrenchStub(t)
	now := time.Date(2031, 10, 4, 9, 0, 0, 0, time.UTC)
	or := platform.NewOpenRouter(stub.srv.URL, "k", stub.srv.Client(), e, func() time.Time { return now })
	tr := New(e, or, platform.Config{TranslateModel: "m", DailySpendUSD: 1}, 5, func() time.Time { return now })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		_, _ = tx.Exec(ctx, `DELETE FROM feed_entries`)
		reader := func(email, locale string) string {
			u := pgtest.User(t, ctx, email)
			_, _ = tx.Exec(ctx, `INSERT INTO profiles (id, display_name, locale, daily_budget) VALUES ($1, 'r', $2, 10)`, u, locale)
			return u.String()
		}
		fr, en := reader("fr@example.com", "fr"), reader("en@example.com", "en")
		delivered, waiting, done := story(t, ctx), story(t, ctx), story(t, ctx)
		entry := func(u string, s int64, state string, on any) {
			_, err := tx.Exec(ctx, `INSERT INTO feed_entries (user_id, story_id, relevance, excluded, score, state, delivered_on) VALUES ($1, $2, 3, 0, 0.5, $3, $4)`, u, s, state, on)
			if err != nil {
				t.Fatal(err)
			}
		}
		entry(fr, delivered, "delivered", "2031-10-04")
		entry(fr, waiting, "candidate", nil)
		entry(fr, done, "delivered", "2031-10-04")
		entry(en, delivered, "delivered", "2031-10-04")
		if err := tr.TranslateStory(ctx, done, i18n.FR); err != nil {
			t.Fatal(err)
		}
		missing, err := Missing(ctx, e, now)
		if err != nil {
			t.Fatal(err)
		}
		want := map[int64]bool{delivered: true, waiting: true}
		if len(missing) != 2 {
			t.Fatalf("missing = %+v", missing)
		}
		for _, m := range missing {
			if !want[m.StoryID] || m.Locale != "fr" {
				t.Errorf("unexpected %+v", m)
			}
		}
	})
}
