package feed

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/learning"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

func TestUsefulSchedulesReviewAndAnswersMoveTheLadder(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	tu := platform.DefaultTuning().Review
	day := time.Date(2031, 11, 1, 9, 0, 0, 0, time.UTC)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		user := pgtest.User(t, ctx, "review@example.com")
		var story, concept int64
		_ = tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category, revision) VALUES ($1, $1, 'space', 1) RETURNING id`, day).Scan(&story)
		_ = tx.QueryRow(ctx, `INSERT INTO concepts (slug, name, definition) VALUES ('review-orbit', 'Orbit', 'd') RETURNING id`).Scan(&concept)
		_, _ = tx.Exec(ctx, `INSERT INTO story_concepts (story_id, concept_id) VALUES ($1, $2)`, story, concept)
		var q1, q2 int64
		_ = tx.QueryRow(ctx, `INSERT INTO review_items (story_id, revision, question, choices, answer_index, explanation) VALUES ($1, 1, 'Q1?', '["a","b","c","d"]', 1, 'E1') RETURNING id`, story).Scan(&q1)
		_ = tx.QueryRow(ctx, `INSERT INTO review_items (story_id, revision, question, choices, answer_index, explanation) VALUES ($1, 1, 'Q2?', '["a","b","c","d"]', 2, 'E2') RETURNING id`, story).Scan(&q2)

		if err := rate(ctx, e, user, story, "useful", day, tu); err != nil {
			t.Fatal(err)
		}
		if due, _ := learning.Due(ctx, tx, user, day, time.UTC, tu); len(due) != 0 {
			t.Errorf("%d questions due on the day of the rating", len(due))
		}
		tomorrow := day.Add(24 * time.Hour)
		due, err := learning.Due(ctx, tx, user, tomorrow, time.UTC, tu)
		if err != nil || len(due) != 2 {
			t.Fatalf("due tomorrow: %d (%v)", len(due), err)
		}

		wrong, err := learning.AnswerReview(ctx, e, user, q1, 0, tomorrow, tu)
		if err != nil || wrong.Correct || wrong.NextDueInDays != 1 || wrong.Explanation != "E1" {
			t.Errorf("wrong answer: %+v (%v)", wrong, err)
		}
		right, _ := learning.AnswerReview(ctx, e, user, q2, 2, tomorrow, tu)
		if !right.Correct || right.NextDueInDays != 3 {
			t.Errorf("right answer: %+v", right)
		}
		next := tomorrow.Add(24 * time.Hour)
		due, _ = learning.Due(ctx, tx, user, next, time.UTC, tu)
		if len(due) != 1 || due[0].ID != q1 {
			t.Errorf("the wrong answer must come back the next day, due = %+v", due)
		}

		states, _ := learning.ConceptStates(ctx, tx, user)
		if states[concept] != learning.StateFamiliar {
			t.Errorf("concept state %d after a correct review", states[concept])
		}
		if _, err := learning.AnswerReview(ctx, e, user, 999999, 0, next, tu); !errors.Is(err, learning.ErrNotDue) {
			t.Errorf("unscheduled question: %v", err)
		}
	})
}

func TestAtMostFiveReviewsADay(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	tu := platform.DefaultTuning().Review
	day := time.Date(2031, 11, 5, 9, 0, 0, 0, time.UTC)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		user := pgtest.User(t, ctx, "five@example.com")
		var story int64
		_ = tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category, revision) VALUES ($1, $1, 'x', 1) RETURNING id`, day).Scan(&story)
		var ids []int64
		for i := range 7 {
			var id int64
			_ = tx.QueryRow(ctx, `INSERT INTO review_items (story_id, revision, question, choices, answer_index, explanation) VALUES ($1, 1, $2, '["a","b","c","d"]', 0, 'e') RETURNING id`, story, "Q"+string(rune('a'+i))).Scan(&id)
			ids = append(ids, id)
			_, _ = tx.Exec(ctx, `INSERT INTO review_schedules (user_id, review_item_id, due_at, interval_days) VALUES ($1, $2, $3, 1)`, user, id, day.Add(-time.Hour))
		}
		due, _ := learning.Due(ctx, tx, user, day, time.UTC, tu)
		if len(due) != tu.MaxDueCard {
			t.Fatalf("%d due, want %d", len(due), tu.MaxDueCard)
		}
		for _, d := range due[:3] {
			_, _ = learning.AnswerReview(ctx, e, user, d.ID, 0, day, tu)
		}
		due, _ = learning.Due(ctx, tx, user, day, time.UTC, tu)
		if len(due) != tu.MaxDueCard-3 {
			t.Errorf("%d due after 3 answers, want %d", len(due), tu.MaxDueCard-3)
		}
	})
}
