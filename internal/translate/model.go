// Package translate translates stories and articles into each reader's
// language and caches the result.
package translate

import (
	"github.com/alternayte/drel"
	"github.com/google/uuid"
)

type TranslationKind string

const (
	// KindStory is a story entry: summary, claims, concepts and review.
	KindStory TranslationKind = "story"
	// KindArticle is the extracted text of one source.
	KindArticle TranslationKind = "article"
)

// ConceptText is one concept of a story in another language.
type ConceptText struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

// ReviewText is one review question in another language.
type ReviewText struct {
	ID          int64    `json:"id"`
	Question    string   `json:"question"`
	Choices     []string `json:"choices"`
	Explanation string   `json:"explanation"`
}

// Fields holds the translated text of one object; a story fills the entry
// fields, an article fills Paragraphs.
type Fields struct {
	Headline   string        `json:"headline,omitempty"`
	Summary    string        `json:"summary,omitempty"`
	Why        string        `json:"why_it_matters,omitempty"`
	New        string        `json:"what_is_new,omitempty"`
	Claims     []string      `json:"claims,omitempty"`
	Concepts   []ConceptText `json:"concepts,omitempty"`
	Review     []ReviewText  `json:"review,omitempty"`
	Paragraphs []string      `json:"paragraphs,omitempty"`
}

// Translation caches one object in one locale at one revision.
type Translation struct {
	drel.Model[int64]
	Kind        TranslationKind `db:"kind,unique_index=uq_translations"`
	ObjectID    int64           `db:"object_id,unique_index=uq_translations"`
	Revision    int             `db:"revision,unique_index=uq_translations"`
	Locale      string          `db:"locale,unique_index=uq_translations"`
	Fields      Fields          `db:"fields"`
	WriterModel string          `db:"model"`
	// RequestedBy is the reader who asked for an article translation; the
	// daily limit counts it.
	RequestedBy *uuid.UUID `db:"requested_by,references=auth_users.id,on_delete=set_null"`
}
