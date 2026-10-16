package learning

import (
	"context"

	"github.com/alternayte/drel"
	"github.com/google/uuid"
)

type ConceptState int

const (
	StateNone ConceptState = iota
	// StateMet: the reader opened a story with the concept.
	StateMet
	// StateFamiliar: the reader marked such a story useful or knew, or
	// answered one of its review questions correctly.
	StateFamiliar
	// StateMastered: three correct reviews on separate days, or knew on
	// three stories.
	StateMastered
)

// ConceptStates derives the state of every concept the reader has an event
// for. Nothing is stored: the events are the record.
func ConceptStates(ctx context.Context, tx *drel.Tx, userID uuid.UUID) (map[int64]ConceptState, error) {
	rows, err := tx.Query(ctx, `
		SELECT sc.concept_id,
			bool_or(e.kind = 'opened'),
			bool_or(e.kind IN ('useful', 'knew', 'reviewed_correct')),
			count(DISTINCT (e.created_at AT TIME ZONE 'UTC')::date) FILTER (WHERE e.kind = 'reviewed_correct'),
			count(DISTINCT e.story_id) FILTER (WHERE e.kind = 'knew')
		FROM learning_events e
		JOIN story_concepts sc ON sc.story_id = e.story_id
		WHERE e.user_id = $1
		GROUP BY sc.concept_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]ConceptState{}
	for rows.Next() {
		var id int64
		var opened, familiar bool
		var correctDays, knewStories int
		if err := rows.Scan(&id, &opened, &familiar, &correctDays, &knewStories); err != nil {
			return nil, err
		}
		switch {
		case correctDays >= 3 || knewStories >= 3:
			out[id] = StateMastered
		case familiar:
			out[id] = StateFamiliar
		case opened:
			out[id] = StateMet
		}
	}
	return out, rows.Err()
}
