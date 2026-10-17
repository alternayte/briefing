package learning

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/alternayte/drel"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/platform"
)

// ErrNotDue reports an answer to a question that is not scheduled for the
// reader.
var ErrNotDue = errors.New("this question is not due for the reader")

// DueItem is one review question the reader should answer now.
type DueItem struct {
	ID          int64    `json:"id"`
	StoryID     int64    `json:"story_id"`
	Headline    string   `json:"story_headline"`
	Question    string   `json:"question"`
	Choices     []string `json:"choices"`
	explanation string
	answer      int
}

// Due returns the questions due now, at most as many as the reader's day,
// which starts at midnight in loc, has left.
func Due(ctx context.Context, tx *drel.Tx, userID uuid.UUID, now time.Time, loc *time.Location, t platform.ReviewTuning) ([]DueItem, error) {
	var answered int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM learning_events WHERE user_id = $1
		AND kind IN ('reviewed_correct', 'reviewed_wrong') AND created_at >= $2`, userID, accounts.StartOfDay(now, loc)).Scan(&answered); err != nil {
		return nil, err
	}
	left := t.MaxDueCard - answered
	if left <= 0 {
		return nil, nil
	}
	rows, err := tx.Query(ctx, `
		SELECT r.id, r.story_id, COALESCE(m.headline, ''), r.question, r.choices, r.explanation, r.answer_index
		FROM review_schedules s JOIN review_items r ON r.id = s.review_item_id
		LEFT JOIN summaries m ON m.story_id = r.story_id AND m.revision = r.revision
		WHERE s.user_id = $1 AND s.due_at <= $2
		ORDER BY s.due_at, r.id LIMIT $3`, userID, now, left)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DueItem
	for rows.Next() {
		var d DueItem
		var choices []byte
		if err := rows.Scan(&d.ID, &d.StoryID, &d.Headline, &d.Question, &choices, &d.explanation, &d.answer); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(choices, &d.Choices); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Answer is the outcome of one answer.
type Answer struct {
	ReviewItemID  int64  `json:"review_item_id"`
	Correct       bool   `json:"correct"`
	AnswerIndex   int    `json:"answer_index"`
	Explanation   string `json:"explanation"`
	NextDueInDays int    `json:"next_due_in_days"`
}

// AnswerReview records an answer to a scheduled question: an event for the
// concept states, and the next step on the interval ladder.
func AnswerReview(ctx context.Context, engine *drel.Engine, userID uuid.UUID, reviewItemID int64, choice int, now time.Time, t platform.ReviewTuning) (Answer, error) {
	a := Answer{ReviewItemID: reviewItemID}
	err := engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		var storyID int64
		var interval int
		err := tx.QueryRow(ctx, `
			SELECT r.story_id, r.answer_index, r.explanation, s.interval_days
			FROM review_schedules s JOIN review_items r ON r.id = s.review_item_id
			WHERE s.user_id = $1 AND s.review_item_id = $2`, userID, reviewItemID).Scan(&storyID, &a.AnswerIndex, &a.Explanation, &interval)
		if isNoRows(err) {
			return ErrNotDue
		}
		if err != nil {
			return err
		}
		a.Correct = choice == a.AnswerIndex
		a.NextDueInDays = nextInterval(interval, a.Correct, t.IntervalsDays)
		kind, result := EventReviewedWrong, "wrong"
		if a.Correct {
			kind, result = EventReviewedCorrect, "correct"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO learning_events (user_id, story_id, kind, payload, created_at, updated_at)
			VALUES ($1, $2, $3, jsonb_build_object('review_item_id', $4::bigint, 'choice', $5::int), $6, $6)`,
			userID, storyID, kind, reviewItemID, choice, now); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE review_schedules SET interval_days = $3, due_at = $4::timestamptz + make_interval(days => $3),
				streak = CASE WHEN $5 THEN streak + 1 ELSE 0 END, last_result = $6, updated_at = $4
			WHERE user_id = $1 AND review_item_id = $2`, userID, reviewItemID, a.NextDueInDays, now, a.Correct, result)
		return err
	})
	return a, err
}
