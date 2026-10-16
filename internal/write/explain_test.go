package write

import (
	"context"
	"encoding/json"
	"errors"
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

func TestExplainUsesWhatTheReaderKnowsAndCaches(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	var calls int
	var system, user string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req struct{ Messages []platform.ChatMessage }
		_ = json.NewDecoder(r.Body).Decode(&req)
		system, user = req.Messages[0].Content, req.Messages[1].Content
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"finish_reason": "stop", "message": map[string]string{"content": "Une explication."}}},
			"usage":   map[string]any{"prompt_tokens": 300, "completion_tokens": 200, "cost": 0.0003},
		})
	}))
	defer srv.Close()
	now := time.Date(2031, 9, 1, 10, 0, 0, 0, time.UTC)
	or := platform.NewOpenRouter(srv.URL, "k", srv.Client(), e, func() time.Time { return now })
	ex := NewExplainer(e, or, platform.Config{WriteModel: "m", DailySpendUSD: 1}, 2, func() time.Time { return now })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		user1 := pgtest.User(t, ctx, "explain@example.com")
		story := storyWith(t, ctx, 1, 4000)
		_, _ = tx.Exec(ctx, `INSERT INTO summaries (story_id, revision, headline, summary, why_it_matters, what_is_new, claims, confidence, model, prompt_version, publisher_count, text_items)
			VALUES ($1, 1, 'Probe lands', 'It landed.', 'w', 'n', '[]', 'medium', 'm', 'v', 1, 1)`, story)
		_, _ = tx.Exec(ctx, `UPDATE stories SET revision = 1 WHERE id = $1`, story)
		var known, fresh int64
		_ = tx.QueryRow(ctx, `INSERT INTO concepts (slug, name, definition) VALUES ('orbit', 'Orbit', 'd') RETURNING id`).Scan(&known)
		_ = tx.QueryRow(ctx, `INSERT INTO concepts (slug, name, definition) VALUES ('regolith', 'Regolith', 'd') RETURNING id`).Scan(&fresh)
		_, _ = tx.Exec(ctx, `INSERT INTO story_concepts (story_id, concept_id) VALUES ($1, $2), ($1, $3)`, story, known, fresh)
		var other int64
		_ = tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category) VALUES (now(), now(), 'x') RETURNING id`).Scan(&other)
		_, _ = tx.Exec(ctx, `INSERT INTO story_concepts (story_id, concept_id) VALUES ($1, $2)`, other, known)
		_, _ = tx.Exec(ctx, `INSERT INTO learning_events (user_id, story_id, kind, payload) VALUES ($1, $2, 'useful', '{}')`, user1, other)

		text, err := ex.Explain(ctx, user1, story, i18n.FR)
		if err != nil || text != "Une explication." {
			t.Fatalf("explain = %q, %v", text, err)
		}
		if !strings.Contains(system, "French") || !strings.Contains(user, "Known: Orbit") || !strings.Contains(user, "New: Regolith") {
			t.Errorf("prompt: %q / %q", system, user)
		}
		if _, err := ex.Explain(ctx, user1, story, i18n.FR); err != nil || calls != 1 {
			t.Errorf("second explain made %d calls (%v)", calls, err)
		}
		if _, err := ex.Explain(ctx, user1, story, i18n.EN); err != nil || calls != 2 {
			t.Errorf("an English explanation must be its own: %d calls", calls)
		}
		if _, err := ex.Explain(ctx, user1, other, i18n.EN); !errors.Is(err, ErrNoEntry) {
			t.Errorf("story without entry: %v", err)
		}
		var events int
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM learning_events WHERE user_id = $1 AND kind = 'explained'`, user1).Scan(&events)
		if events != 2 {
			t.Errorf("%d explained events", events)
		}

		third := storyWith(t, ctx, 1, 4000)
		_, _ = tx.Exec(ctx, `INSERT INTO summaries (story_id, revision, headline, summary, why_it_matters, what_is_new, claims, confidence, model, prompt_version, publisher_count, text_items)
			VALUES ($1, 1, 'h', 's', 'w', 'n', '[]', 'medium', 'm', 'v', 1, 1)`, third)
		if _, err := ex.Explain(ctx, user1, third, i18n.FR); !errors.Is(err, ErrExplainLimit) {
			t.Errorf("third explanation of the day: %v", err)
		}
	})
}

func TestExplainStopsAtTheSpendCap(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	now := time.Date(2031, 9, 2, 10, 0, 0, 0, time.UTC)
	or := platform.NewOpenRouter("http://127.0.0.1:1", "k", http.DefaultClient, e, func() time.Time { return now })
	ex := NewExplainer(e, or, platform.Config{WriteModel: "m", DailySpendUSD: 0.01}, 20, func() time.Time { return now })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		u := pgtest.User(t, ctx, "cap@example.com")
		story := storyWith(t, ctx, 1, 4000)
		_, _ = tx.Exec(ctx, `INSERT INTO summaries (story_id, revision, headline, summary, why_it_matters, what_is_new, claims, confidence, model, prompt_version, publisher_count, text_items)
			VALUES ($1, 1, 'h', 's', 'w', 'n', '[]', 'medium', 'm', 'v', 1, 1)`, story)
		_, _ = tx.Exec(ctx, `INSERT INTO llm_usage (day, provider, stage, model, calls, cost_usd) VALUES ('2031-09-02', 'openrouter', 'write', 'm', 1, 0.02)`)
		if _, err := ex.Explain(ctx, u, story, i18n.EN); !errors.Is(err, ErrCapReached) {
			t.Errorf("err = %v", err)
		}
	})
}
