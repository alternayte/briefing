package stories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alternayte/drel"

	"github.com/alternayte/briefing/internal/decide"
	"github.com/alternayte/briefing/internal/ingest"
	"github.com/alternayte/briefing/internal/platform"
)

// Clusterer puts each embedded item into the open story about the same event,
// or opens a new story.
type Clusterer struct {
	engine  *drel.Engine
	decider *decide.Decider
	tuning  platform.ClusterTuning
	now     func() time.Time
}

func NewClusterer(engine *drel.Engine, d *decide.Decider, t platform.Tuning, now func() time.Time) *Clusterer {
	return &Clusterer{engine: engine, decider: d, tuning: t.Cluster, now: now}
}

var sameEvent = decide.Questions{
	"same_event": decide.Noul("Do the story and the new item report the same event?",
		"They report the same event, release, finding or incident.",
		"They report different events, even if the topic is related."),
}

// Cluster places each item and returns the ids of the stories it touched. It
// runs the items one by one, so an item can join a story that an earlier item
// in the same batch opened. Each placement hands its story to next in the
// same transaction.
func (c *Clusterer) Cluster(ctx context.Context, itemIDs []int64, next platform.Next) ([]int64, error) {
	var touched []int64
	for _, id := range itemIDs {
		storyID, err := c.place(ctx, id, next)
		if err != nil {
			return touched, fmt.Errorf("cluster item %d: %w", id, err)
		}
		if storyID != 0 {
			touched = append(touched, storyID)
		}
	}
	return touched, nil
}

type candidate struct {
	storyID    int64
	similarity float64
}

func (c *Clusterer) place(ctx context.Context, itemID int64, next platform.Next) (int64, error) {
	it, err := c.unplaced(ctx, itemID)
	if err != nil || it == nil {
		return 0, err
	}
	now := c.now().UTC()
	near, err := c.nearest(ctx, it, now)
	if err != nil {
		return 0, err
	}
	join := near != nil && near.similarity >= c.tuning.JoinHigh
	if near != nil && !join && near.similarity >= c.tuning.JoinLow {
		if join, err = c.askSameEvent(ctx, near.storyID, it); err != nil {
			return 0, err
		}
	}
	var storyID int64
	err = c.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		if join {
			storyID = near.storyID
		} else {
			s := &Story{OpenedAt: now, LastItemAt: now, Category: it.Triage.Category, Status: StoryOpen, Centroid: it.Embedding}
			drel.NewTxRepository(tx, StoryMeta).Add(s)
			if err := tx.SaveChanges(ctx); err != nil {
				return err
			}
			storyID = s.ID()
		}
		if _, err := tx.Exec(ctx, `INSERT INTO story_items (story_id, item_id) VALUES ($1, $2) ON CONFLICT (item_id) DO NOTHING`, storyID, it.ID()); err != nil {
			return err
		}
		if err := refresh(ctx, tx, storyID, now); err != nil || next == nil {
			return err
		}
		return next(ctx, []int64{storyID})
	})
	return storyID, err
}

// unplaced returns the item when it is kept, embedded and in no story.
func (c *Clusterer) unplaced(ctx context.Context, id int64) (*ingest.Item, error) {
	var it *ingest.Item
	err := c.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		found, err := drel.NewTxRepository(tx, ingest.ItemMeta).Find(ctx, id)
		if errors.Is(err, drel.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if found.Status != ingest.ItemKept || found.Embedding.IsZero() || found.Triage == nil {
			return nil
		}
		var placed bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM story_items WHERE item_id = $1)`, id).Scan(&placed); err != nil {
			return err
		}
		if !placed {
			it = found
		}
		return nil
	})
	return it, err
}

func (c *Clusterer) nearest(ctx context.Context, it *ingest.Item, now time.Time) (*candidate, error) {
	vec, err := it.Embedding.Value()
	if err != nil {
		return nil, err
	}
	var near *candidate
	err = c.engine.WithTx(ctx, func(ctx context.Context) error {
		var cand candidate
		err := drel.MustFromContext(ctx).QueryRow(ctx, `
			SELECT id, 1 - (centroid <=> $1::vector)
			FROM stories
			WHERE status = 'open' AND last_item_at >= $2 AND vector_dims(centroid) = vector_dims($1::vector)
			ORDER BY centroid <=> $1::vector
			LIMIT 1`, vec, now.Add(-c.tuning.OpenWindow)).Scan(&cand.storyID, &cand.similarity)
		if err == nil {
			near = &cand
			return nil
		}
		if isNoRows(err) {
			return nil
		}
		return err
	})
	return near, err
}

func (c *Clusterer) askSameEvent(ctx context.Context, storyID int64, it *ingest.Item) (bool, error) {
	var headline, snippet string
	err := c.engine.WithTx(ctx, func(ctx context.Context) error {
		return drel.MustFromContext(ctx).QueryRow(ctx, `
			SELECT i.title, i.snippet FROM story_items si JOIN items i ON i.id = si.item_id
			WHERE si.story_id = $1
			ORDER BY (i.triage->>'significance')::float8 DESC NULLS LAST, i.id
			LIMIT 1`, storyID).Scan(&headline, &snippet)
	})
	if err != nil {
		return false, err
	}
	state := fmt.Sprintf("Story: %s\n%s\n\nNew item: %s\n%s", headline, snippet, it.Title, it.Snippet)
	ans, err := c.decider.Ask(ctx, "same_event", state, sameEvent)
	if err != nil {
		return false, err
	}
	return ans["same_event"].Noul >= c.tuning.SameEventMin, nil
}

// refresh recomputes a story from its items: the centroid, the scores (the
// maximum over items), the category of its most significant item, and the
// count of distinct publishers.
func refresh(ctx context.Context, tx *drel.Tx, storyID int64, now time.Time) error {
	rows, err := tx.Query(ctx, `SELECT i.canonical_url FROM story_items si JOIN items i ON i.id = si.item_id WHERE si.story_id = $1`, storyID)
	if err != nil {
		return err
	}
	publishers := map[string]bool{}
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			rows.Close()
			return err
		}
		publishers[ingest.RegistrableDomain(u)] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE stories s SET
			centroid = agg.centroid,
			significance = agg.significance,
			learning = agg.learning,
			sensational = agg.sensational,
			category = agg.category,
			publisher_count = $2,
			last_item_at = GREATEST(s.last_item_at, $3),
			updated_at = $3
		FROM (
			SELECT avg(i.embedding) AS centroid,
				max((i.triage->>'significance')::float8) AS significance,
				max((i.triage->>'learning')::float8) AS learning,
				max((i.triage->>'sensational')::float8) AS sensational,
				(array_agg(i.triage->>'category' ORDER BY (i.triage->>'significance')::float8 DESC NULLS LAST, i.id))[1] AS category
			FROM story_items si JOIN items i ON i.id = si.item_id
			WHERE si.story_id = $1
		) agg
		WHERE s.id = $1`, storyID, len(publishers), now)
	return err
}

// CloseStale closes open stories with no new item inside the window.
func (c *Clusterer) CloseStale(ctx context.Context) (int64, error) {
	var n int64
	err := c.engine.WithTx(ctx, func(ctx context.Context) (err error) {
		n, err = drel.MustFromContext(ctx).Exec(ctx,
			`UPDATE stories SET status = 'closed', updated_at = $1 WHERE status = 'open' AND last_item_at < $2`,
			c.now().UTC(), c.now().UTC().Add(-c.tuning.OpenWindow))
		return err
	})
	return n, err
}
