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
	// Unfetched are shortlisted stories with items still waiting for text.
	Unfetched []int64
}

// FindStalled returns at most limit stalled items per step.
func FindStalled(ctx context.Context, engine *drel.Engine, now time.Time, grace time.Duration, limit int, minSignificance float64) (Stalled, error) {
	var s Stalled
	before := now.Add(-grace)
	err := engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		for _, q := range []struct {
			dst  *[]int64
			sql  string
			args []any
		}{
			{&s.New, `SELECT id FROM items WHERE status = 'new' AND updated_at < $1 ORDER BY id LIMIT $2`, []any{before, limit}},
			{&s.Passed, `SELECT id FROM items WHERE status = 'passed' AND updated_at < $1 ORDER BY id LIMIT $2`, []any{before, limit}},
			{&s.Kept, `SELECT id FROM items WHERE status = 'kept' AND embed_model = '' AND updated_at < $1 ORDER BY id LIMIT $2`, []any{before, limit}},
			{&s.Unplaced, `SELECT i.id FROM items i WHERE i.status = 'kept' AND i.embed_model <> '' AND i.updated_at < $1
				AND NOT EXISTS (SELECT 1 FROM story_items si WHERE si.item_id = i.id) ORDER BY i.id LIMIT $2`, []any{before, limit}},
			{&s.Unfetched, `SELECT DISTINCT s.id FROM stories s JOIN story_items si ON si.story_id = s.id JOIN items i ON i.id = si.item_id
				WHERE s.significance >= $3 AND i.text_source = 'pending' AND si.created_at < $1 ORDER BY s.id LIMIT $2`, []any{before, limit, minSignificance}},
		} {
			rows, err := tx.Query(ctx, q.sql, q.args...)
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

// Shortlist returns the stories among ids that are significant enough to be
// fetched and written for every reader.
func Shortlist(ctx context.Context, engine *drel.Engine, ids []int64, minSignificance float64) ([]int64, error) {
	var out []int64
	err := engine.WithTx(ctx, func(ctx context.Context) error {
		rows, err := drel.MustFromContext(ctx).Query(ctx,
			`SELECT id FROM stories WHERE id = ANY($1) AND significance >= $2 ORDER BY id`, ids, minSignificance)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			out = append(out, id)
		}
		return rows.Err()
	})
	return out, err
}
