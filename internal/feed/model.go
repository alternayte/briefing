// Package feed ranks stories for each user and serves the feed, the story
// view, feedback, onboarding and settings.
package feed

import (
	"time"

	"github.com/alternayte/drel"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/platform"
)

// Interest is one topic a user wants, or with Negative, does not want.
type Interest struct {
	drel.Model[int64]
	UserID     uuid.UUID       `db:"user_id,references=auth_users.id,on_delete=cascade,index=idx_interests_user"`
	Text       string          `db:"text"`
	Negative   bool            `db:"negative,default=false"`
	Weight     float64         `db:"weight,default=1"`
	Embedding  platform.Vector `db:"embedding,type=vector"`
	EmbedModel string          `db:"embed_model,default=''"`
}

type EntryState string

const (
	// EntryCandidate is scored but not delivered.
	EntryCandidate EntryState = "candidate"
	// EntryDelivered is in the user's feed and unread.
	EntryDelivered EntryState = "delivered"
	// EntrySeen was opened or marked by the user.
	EntrySeen EntryState = "seen"
	// EntryDone was taken off Today by the reader: Done, Clear read, "Knew
	// it" or "Not relevant". A delivered one still counts toward the budget.
	EntryDone EntryState = "done"
)

// Entry is one story in one user's feed.
type Entry struct {
	drel.Model[int64] `db:"table=feed_entries,index=idx_feed_entries_user_state[user_id,state]"`
	UserID            uuid.UUID  `db:"user_id,references=auth_users.id,on_delete=cascade,unique_index=uq_feed_entries"`
	StoryID           int64      `db:"story_id,references=stories.id,on_delete=cascade,unique_index=uq_feed_entries"`
	Relevance         float64    `db:"relevance"`
	Excluded          float64    `db:"excluded"`
	Score             float64    `db:"score"`
	Reason            string     `db:"reason,default=''"`
	ShownRevision     int        `db:"shown_revision,default=0"`
	DeliveredOn       *time.Time `db:"delivered_on,type=date"`
	// DeliveredAt is when the entry entered the feed; an open page counts
	// the entries delivered after it loaded.
	DeliveredAt *time.Time `db:"delivered_at"`
	State       EntryState `db:"state,default=candidate"`
}
