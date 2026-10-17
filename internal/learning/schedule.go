package learning

import (
	"context"
	"time"

	"github.com/alternayte/drel"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/platform"
)

// ReviewSchedule is when a reader next sees one review question.
type ReviewSchedule struct {
	drel.Model[int64] `db:"index=idx_review_schedules_due[user_id,due_at]"`
	UserID            uuid.UUID `db:"user_id,references=auth_users.id,on_delete=cascade,unique_index=uq_review_schedules"`
	ReviewItemID      int64     `db:"review_item_id,references=review_items.id,on_delete=cascade,unique_index=uq_review_schedules"`
	DueAt             time.Time `db:"due_at"`
	IntervalDays      int       `db:"interval_days"`
	Streak            int       `db:"streak,default=0"`
	LastResult        string    `db:"last_result,default=''"`
}

// ScheduleStory schedules the review questions of the story's latest
// revision for the reader, first due after the shortest interval. It runs in
// the caller's transaction and leaves existing schedules alone.
func ScheduleStory(ctx context.Context, tx *drel.Tx, userID uuid.UUID, storyID int64, now time.Time, t platform.ReviewTuning) error {
	first := t.IntervalsDays[0]
	_, err := tx.Exec(ctx, `
		INSERT INTO review_schedules (user_id, review_item_id, due_at, interval_days)
		SELECT $1, r.id, $3::timestamptz + make_interval(days => $4), $4
		FROM review_items r
		WHERE r.story_id = $2 AND r.revision = (SELECT max(revision) FROM review_items WHERE story_id = $2)
		ON CONFLICT (user_id, review_item_id) DO NOTHING`, userID, storyID, now, first)
	return err
}

// UnscheduleStory removes the reader's review questions of a story that they
// have not answered yet. Answered ones keep their history.
func UnscheduleStory(ctx context.Context, tx *drel.Tx, userID uuid.UUID, storyID int64) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM review_schedules rs USING review_items r
		WHERE rs.review_item_id = r.id AND r.story_id = $2 AND rs.user_id = $1 AND rs.last_result = ''`, userID, storyID)
	return err
}

// nextInterval climbs the ladder after a correct answer and returns to the
// first step after a wrong one.
func nextInterval(current int, correct bool, ladder []int) int {
	if !correct {
		return ladder[0]
	}
	for _, d := range ladder {
		if d > current {
			return d
		}
	}
	return ladder[len(ladder)-1]
}
