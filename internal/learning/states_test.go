package learning

import (
	"context"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/platform/pgtest"
)

func TestConceptStatesFollowTheEvents(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		user := pgtest.User(t, ctx, "learner@example.com")
		concept := func(slug string) int64 {
			var id int64
			_ = tx.QueryRow(ctx, `INSERT INTO concepts (slug, name, definition) VALUES ($1, $1, 'd') RETURNING id`, slug).Scan(&id)
			return id
		}
		story := func(concepts ...int64) int64 {
			var id int64
			_ = tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category) VALUES (now(), now(), 'x') RETURNING id`).Scan(&id)
			for _, c := range concepts {
				_, _ = tx.Exec(ctx, `INSERT INTO story_concepts (story_id, concept_id) VALUES ($1, $2)`, id, c)
			}
			return id
		}
		event := func(s int64, kind EventKind, at time.Time) {
			if _, err := tx.Exec(ctx, `INSERT INTO learning_events (user_id, story_id, kind, payload, created_at) VALUES ($1, $2, $3, '{}', $4)`, user, s, kind, at); err != nil {
				t.Fatal(err)
			}
		}
		met, familiar, byReview, byKnew, sameDay := concept("met"), concept("familiar"), concept("by-review"), concept("by-knew"), concept("same-day")
		day := time.Date(2031, 1, 1, 9, 0, 0, 0, time.UTC)

		event(story(met), EventOpened, day)
		event(story(familiar), EventUseful, day)
		s := story(byReview)
		for i := range 3 {
			event(s, EventReviewedCorrect, day.AddDate(0, 0, i))
		}
		for range 3 {
			event(story(byKnew), EventKnew, day)
		}
		s2 := story(sameDay)
		for i := range 3 {
			event(s2, EventReviewedCorrect, day.Add(time.Duration(i)*time.Hour))
		}

		states, err := ConceptStates(ctx, tx, user)
		if err != nil {
			t.Fatal(err)
		}
		want := map[int64]ConceptState{met: StateMet, familiar: StateFamiliar, byReview: StateMastered, byKnew: StateMastered, sameDay: StateFamiliar}
		for id, w := range want {
			if states[id] != w {
				t.Errorf("concept %d: state %d, want %d", id, states[id], w)
			}
		}
		if _, ok := states[concept("untouched")]; ok {
			t.Error("a concept with no events has a state")
		}
	})
}
