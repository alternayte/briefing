package feed

import (
	"context"
	"errors"
	"time"

	"github.com/alternayte/drel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/alternayte/briefing/internal/learning"
	"github.com/alternayte/briefing/internal/platform"
)

const (
	calibrationStories = 10
	weightStep         = 0.1
	minWeight          = 0.2
	maxWeight          = 2.0
)

// Ratings of calibration stories and feedback kinds, as the forms send them.
var ratingKinds = map[string]learning.EventKind{
	"useful":     learning.EventUseful,
	"knew":       learning.EventKnew,
	"irrelevant": learning.EventIrrelevant,
	"misleading": learning.EventMisleading,
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

// rate records a rating once per story and kind. Useful also schedules the
// story's review questions. The first useful raises, and the first
// irrelevant lowers, the weight of the interest nearest to the story; the
// event keeps the change it made, so unrate can reverse it. Knew and
// irrelevant take the entry off Today (state done) and keep the state they
// replaced. Useful and misleading leave the story where it is, as seen.
func rate(ctx context.Context, engine *drel.Engine, userID uuid.UUID, storyID int64, rating string, now time.Time, review platform.ReviewTuning) error {
	kind, ok := ratingKinds[rating]
	if !ok {
		return nil
	}
	return engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		var same, opinion bool
		if err := tx.QueryRow(ctx, `SELECT
			EXISTS (SELECT 1 FROM learning_events WHERE user_id = $1 AND story_id = $2 AND kind = $3),
			EXISTS (SELECT 1 FROM learning_events WHERE user_id = $1 AND story_id = $2 AND kind IN ('useful', 'irrelevant'))`,
			userID, storyID, kind).Scan(&same, &opinion); err != nil || same {
			return err
		}
		payload := map[string]any{}
		if dismisses(kind) {
			var prior string
			err := tx.QueryRow(ctx, `
				UPDATE feed_entries f SET state = 'done', shown_revision = s.revision, updated_at = now()
				FROM stories s, (SELECT id, state FROM feed_entries WHERE user_id = $1 AND story_id = $2 FOR UPDATE) old
				WHERE s.id = f.story_id AND f.id = old.id
				RETURNING old.state::text`, userID, storyID).Scan(&prior)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			payload["prior_state"] = prior
		} else if _, err := tx.Exec(ctx, `UPDATE feed_entries SET state = 'seen', updated_at = now() WHERE user_id = $1 AND story_id = $2 AND state = 'delivered'`,
			userID, storyID); err != nil {
			return err
		}
		if kind == learning.EventUseful {
			if err := learning.ScheduleStory(ctx, tx, userID, storyID, now, review); err != nil {
				return err
			}
		}
		step := 0.0
		switch kind {
		case learning.EventUseful:
			step = weightStep
		case learning.EventIrrelevant:
			step = -weightStep
		}
		if step != 0 && !opinion {
			var interestID int64
			var delta float64
			err := tx.QueryRow(ctx, `
				WITH target AS (
					SELECT i.id, i.weight FROM interests i, stories s
					WHERE i.user_id = $1 AND NOT i.negative AND s.id = $2 AND i.embedding IS NOT NULL
						AND vector_dims(i.embedding) = vector_dims(s.centroid)
					ORDER BY i.embedding <=> s.centroid LIMIT 1)
				UPDATE interests SET weight = LEAST($4, GREATEST($5, interests.weight + $3)), updated_at = now()
				FROM target WHERE interests.id = target.id
				RETURNING interests.id, interests.weight - target.weight`,
				userID, storyID, step, maxWeight, minWeight).Scan(&interestID, &delta)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if err == nil {
				payload["interest_id"], payload["delta"] = interestID, delta
			}
		}
		_, err := tx.Exec(ctx, `INSERT INTO learning_events (user_id, story_id, kind, payload) VALUES ($1, $2, $3, $4)`,
			userID, storyID, kind, payload)
		return err
	})
}

// unrate removes a rating and reverses what rate did: the interest weight
// change, the unanswered review questions of a useful story, and for knew
// or irrelevant, the entry's state when no other of the two remains. A
// rating that is not there changes nothing.
func unrate(ctx context.Context, engine *drel.Engine, userID uuid.UUID, storyID int64, rating string) error {
	kind, ok := ratingKinds[rating]
	if !ok {
		return nil
	}
	return engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		rows, err := tx.Query(ctx, `DELETE FROM learning_events WHERE user_id = $1 AND story_id = $2 AND kind = $3 RETURNING payload`,
			userID, storyID, kind)
		if err != nil {
			return err
		}
		var payloads []ratingPayload
		for rows.Next() {
			var p ratingPayload
			if err := rows.Scan(&p); err != nil {
				rows.Close()
				return err
			}
			payloads = append(payloads, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil || len(payloads) == 0 {
			return err
		}
		for _, p := range payloads {
			if p.InterestID == 0 || p.Delta == 0 {
				continue
			}
			if _, err := tx.Exec(ctx, `UPDATE interests SET weight = LEAST($4, GREATEST($5, weight - $3)), updated_at = now() WHERE id = $2 AND user_id = $1`,
				userID, p.InterestID, p.Delta, maxWeight, minWeight); err != nil {
				return err
			}
		}
		if kind == learning.EventUseful {
			if err := learning.UnscheduleStory(ctx, tx, userID, storyID); err != nil {
				return err
			}
		}
		if !dismisses(kind) {
			return nil
		}
		prior := payloads[0].PriorState
		if prior == "" {
			prior = string(EntrySeen)
		}
		_, err = tx.Exec(ctx, `
			UPDATE feed_entries SET state = CASE WHEN $3 = 'seen' AND delivered_on IS NULL THEN 'candidate' ELSE $3 END::entrystate, updated_at = now()
			WHERE user_id = $1 AND story_id = $2 AND state = 'done' AND NOT EXISTS (
				SELECT 1 FROM learning_events WHERE user_id = $1 AND story_id = $2 AND kind IN ('knew', 'irrelevant'))`,
			userID, storyID, prior)
		return err
	})
}

// ratingPayload is what rate keeps in a rating event to undo it.
type ratingPayload struct {
	InterestID int64   `json:"interest_id"`
	Delta      float64 `json:"delta"`
	PriorState string  `json:"prior_state"`
}

// dismisses reports whether a rating takes the story off Today.
func dismisses(k learning.EventKind) bool {
	return k == learning.EventKnew || k == learning.EventIrrelevant
}
