package stories

import (
	"context"
	"time"

	"github.com/alternayte/drel"
)

// Stalled lists items that wait at one pipeline step longer than grace. A
// step enqueues the next one after its commit, so a crash in between leaves
// items here until the sweep hands them on.
type Stalled struct {
	New      []int64
	Passed   []int64
	Kept     []int64
	Unplaced []int64
}

// FindStalled returns at most limit stalled items per step.
func FindStalled(ctx context.Context, engine *drel.Engine, now time.Time, grace time.Duration, limit int) (Stalled, error) {
	var s Stalled
	before := now.Add(-grace)
	err := engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		for _, q := range []struct {
			dst *[]int64
			sql string
		}{
			{&s.New, `SELECT id FROM items WHERE status = 'new' AND updated_at < $1 ORDER BY id LIMIT $2`},
			{&s.Passed, `SELECT id FROM items WHERE status = 'passed' AND updated_at < $1 ORDER BY id LIMIT $2`},
			{&s.Kept, `SELECT id FROM items WHERE status = 'kept' AND embed_model = '' AND updated_at < $1 ORDER BY id LIMIT $2`},
			{&s.Unplaced, `SELECT i.id FROM items i WHERE i.status = 'kept' AND i.embed_model <> '' AND i.updated_at < $1
				AND NOT EXISTS (SELECT 1 FROM story_items si WHERE si.item_id = i.id) ORDER BY i.id LIMIT $2`},
		} {
			rows, err := tx.Query(ctx, q.sql, before, limit)
			if err != nil {
				return err
			}
			for rows.Next() {
				var id int64
				if err := rows.Scan(&id); err != nil {
					rows.Close()
					return err
				}
				*q.dst = append(*q.dst, id)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
		}
		return nil
	})
	return s, err
}
