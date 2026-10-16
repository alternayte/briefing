// Package write turns a story's sources into a summary, concepts and review
// questions, within the daily spend cap.
package write

import (
	"github.com/alternayte/drel"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/platform"
)

type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

// Claim is one fact of a summary and the items that state it.
type Claim struct {
	Text    string  `json:"text"`
	ItemIDs []int64 `json:"item_ids"`
}

// Summary is one revision of a story's entry, in English.
type Summary struct {
	drel.Model[int64]
	StoryID        int64      `db:"story_id,references=stories.id,on_delete=cascade,unique_index=uq_summaries_story_revision"`
	Revision       int        `db:"revision,unique_index=uq_summaries_story_revision"`
	Headline       string     `db:"headline"`
	Summary        string     `db:"summary"`
	WhyItMatters   string     `db:"why_it_matters"`
	WhatIsNew      string     `db:"what_is_new"`
	Claims         []Claim    `db:"claims"`
	Confidence     Confidence `db:"confidence"`
	WriterModel    string     `db:"model"`
	PromptVersion  string     `db:"prompt_version"`
	PublisherCount int        `db:"publisher_count"`
	TextItems      int        `db:"text_items"`
}

// Concept is a reusable idea that stories teach.
type Concept struct {
	drel.Model[int64]
	Slug       string          `db:"slug,unique"`
	Name       string          `db:"name"`
	Definition string          `db:"definition"`
	Embedding  platform.Vector `db:"embedding,type=vector"`
	EmbedModel string          `db:"embed_model,default=''"`
}

// StoryConcept links a story to a concept it uses.
type StoryConcept struct {
	drel.Model[int64]
	StoryID   int64 `db:"story_id,references=stories.id,on_delete=cascade,unique_index=uq_story_concepts"`
	ConceptID int64 `db:"concept_id,references=concepts.id,on_delete=cascade,unique_index=uq_story_concepts,index=idx_story_concepts_concept"`
}

// ReviewItem is one multiple-choice question about a story revision.
type ReviewItem struct {
	drel.Model[int64]
	StoryID     int64    `db:"story_id,references=stories.id,on_delete=cascade,unique_index=uq_review_items"`
	Revision    int      `db:"revision,unique_index=uq_review_items"`
	Question    string   `db:"question,unique_index=uq_review_items"`
	Choices     []string `db:"choices"`
	AnswerIndex int      `db:"answer_index"`
	Explanation string   `db:"explanation"`
}

// WriteAttempt counts the writer calls for a story on one day, so a story
// whose output keeps failing validation stops costing money.
type WriteAttempt struct {
	drel.Model[int64]
	StoryID  int64     `db:"story_id,references=stories.id,on_delete=cascade,unique_index=uq_write_attempts"`
	Day      drel.Date `db:"day,unique_index=uq_write_attempts"`
	Attempts int       `db:"attempts"`
}

// Explanation is a story explained for one reader, in their locale, at one
// revision.
type Explanation struct {
	drel.Model[int64]
	UserID      uuid.UUID `db:"user_id,references=auth_users.id,on_delete=cascade,unique_index=uq_explanations"`
	StoryID     int64     `db:"story_id,references=stories.id,on_delete=cascade,unique_index=uq_explanations"`
	Revision    int       `db:"revision,unique_index=uq_explanations"`
	Locale      string    `db:"locale,unique_index=uq_explanations"`
	Text        string    `db:"text"`
	WriterModel string    `db:"model"`
}
