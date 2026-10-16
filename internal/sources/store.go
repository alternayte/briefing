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
