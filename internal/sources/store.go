package sources

import (
	"context"
	"time"

	"github.com/alternayte/drel"
)

// Find returns one source.
func Find(ctx context.Context, engine *drel.Engine, id int64) (*Source, error) {
	var src *Source
	err := engine.WithTx(ctx, func(ctx context.Context) (err error) {
		src, err = drel.NewTxRepository(drel.MustFromContext(ctx), SourceMeta).Find(ctx, id)
		return err
	})
	return src, err
}

// SaveCheckpoint records a successful poll. It must run inside the
// transaction that stores the poll's items.
func SaveCheckpoint(ctx context.Context, id int64, etag, lastModified string, at time.Time) error {
	_, err := drel.MustFromContext(ctx).Exec(ctx,
		`UPDATE sources SET etag = $2, last_modified = $3, last_ok_at = $4, last_error = '', updated_at = $4 WHERE id = $1`,
		id, etag, lastModified, at)
	return err
}

// RecordError keeps the last poll failure of a source for the admin page.
func RecordError(ctx context.Context, engine *drel.Engine, id int64, cause error) error {
	msg := cause.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	return engine.WithTx(ctx, func(ctx context.Context) error {
		_, err := drel.MustFromContext(ctx).Exec(ctx, `UPDATE sources SET last_error = $2, updated_at = now() WHERE id = $1`, id, msg)
		return err
	})
}

// Due returns the enabled sources whose poll interval has passed since their
// last success. A source that failed waits a full interval after the failure.
func Due(ctx context.Context, engine *drel.Engine, now time.Time) ([]int64, error) {
	var ids []int64
	err := engine.WithTx(ctx, func(ctx context.Context) error {
		rows, err := drel.MustFromContext(ctx).Query(ctx, `
			SELECT id FROM sources
			WHERE enabled
			  AND (last_ok_at IS NULL OR last_ok_at + make_interval(secs => poll_interval_seconds) <= $1)
			  AND (last_error = '' OR updated_at + make_interval(secs => poll_interval_seconds) <= $1)
			ORDER BY id`, now)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	return ids, err
}
