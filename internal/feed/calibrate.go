package feed

import (
	"context"
	"time"

	"github.com/alternayte/drel"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/learning"
)

const (
	calibrationStories = 10
	weightStep         = 0.1
	minWeight          = 0.2
	maxWeight          = 2.0
)

// Ratings of calibration stories, as the form sends them.
var ratingKinds = map[string]learning.EventKind{
	"useful":     learning.EventUseful,
	"knew":       learning.EventKnew,
	"irrelevant": learning.EventIrrelevant,
}

type calibrationStory struct {
	ID       int64
	Headline string
	Summary  string
	Rated    string
}

// calibrationSet returns the most significant recent stories with an entry,
// with the reader's rating of each when there is one.
func calibrationSet(ctx context.Context, tx *drel.Tx, userID uuid.UUID, since time.Time) ([]calibrationStory, error) {
	rows, err := tx.Query(ctx, `
		SELECT s.id, m.headline, m.summary,
			COALESCE((SELECT e.kind::text FROM learning_events e
				WHERE e.user_id = $1 AND e.story_id = s.id AND e.kind IN ('useful', 'knew', 'irrelevant')
				ORDER BY e.id DESC LIMIT 1), '')
		FROM stories s
		JOIN LATERAL (SELECT headline, summary FROM summaries WHERE story_id = s.id ORDER BY revision DESC LIMIT 1) m ON true
		WHERE s.last_item_at >= $2
		ORDER BY s.significance DESC, s.id DESC
		LIMIT $3`, userID, since, calibrationStories)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []calibrationStory
	for rows.Next() {
		var c calibrationStory
		if err := rows.Scan(&c.ID, &c.Headline, &c.Summary, &c.Rated); err != nil {
			return nil, err
		}
		for name, kind := range ratingKinds {
			if string(kind) == c.Rated {
				c.Rated = name
			}
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// rate records a calibration rating once per story. Useful raises, and not
// for me lowers, the weight of the interest nearest to the story; knew is
// only an event, which the novelty penalty and the concept states read.
func rate(ctx context.Context, engine *drel.Engine, userID uuid.UUID, storyID int64, rating string) error {
	kind, ok := ratingKinds[rating]
	if !ok {
		return nil
	}
	return engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		var rated bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM learning_events WHERE user_id = $1 AND story_id = $2
			AND kind IN ('useful', 'knew', 'irrelevant'))`, userID, storyID).Scan(&rated); err != nil || rated {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO learning_events (user_id, story_id, kind, payload) VALUES ($1, $2, $3, '{"via":"calibration"}')`,
			userID, storyID, kind); err != nil {
			return err
		}
		step := 0.0
		switch kind {
		case learning.EventUseful:
			step = weightStep
		case learning.EventIrrelevant:
			step = -weightStep
		}
		if step == 0 {
			return nil
		}
		_, err := tx.Exec(ctx, `
			UPDATE interests SET weight = LEAST($4, GREATEST($5, weight + $3)), updated_at = now()
			WHERE id = (
				SELECT i.id FROM interests i, stories s
				WHERE i.user_id = $1 AND NOT i.negative AND s.id = $2 AND i.embedding IS NOT NULL
					AND vector_dims(i.embedding) = vector_dims(s.centroid)
				ORDER BY i.embedding <=> s.centroid LIMIT 1)`,
			userID, storyID, step, maxWeight, minWeight)
		return err
	})
}
