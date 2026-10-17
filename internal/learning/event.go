// Package learning records what each user knows and schedules their review.
package learning

import (
	"github.com/alternayte/drel"
	"github.com/google/uuid"
)

type EventKind string

const (
	EventOpened          EventKind = "opened"
	EventUseful          EventKind = "useful"
	EventKnew            EventKind = "knew"
	EventIrrelevant      EventKind = "irrelevant"
	EventMisleading      EventKind = "misleading"
	EventExplained       EventKind = "explained"
	EventReviewedCorrect EventKind = "reviewed_correct"
	EventReviewedWrong   EventKind = "reviewed_wrong"
)

// Event is one thing a user did that says something about what they know.
// Events are only added, except a feedback event, which the reader can undo.
type Event struct {
	drel.Model[int64] `db:"table=learning_events,index=idx_learning_events_user_kind[user_id,kind,created_at]"`
	UserID            uuid.UUID      `db:"user_id,references=auth_users.id,on_delete=cascade"`
	StoryID           *int64         `db:"story_id,references=stories.id,on_delete=cascade"`
	ConceptID         *int64         `db:"concept_id,references=concepts.id,on_delete=cascade"`
	Kind              EventKind      `db:"kind"`
	Payload           map[string]any `db:"payload"`
}
