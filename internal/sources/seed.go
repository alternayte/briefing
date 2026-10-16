package sources

import (
	"context"
	"fmt"

	"github.com/alternayte/drel"
)

// Seed inserts the catalog entries and refreshes the title, tier and language
// of existing ones. It keeps the enabled flag and the poll checkpoint, which
// belong to the running app.
func Seed(ctx context.Context, engine *drel.Engine, entries []Entry) error {
	rows := make([]*Source, len(entries))
	for i, e := range entries {
		rows[i] = &Source{
			Kind:                e.Kind,
			URL:                 e.URL,
			Title:               e.Title,
			Tier:                e.Tier,
			Language:            e.Language,
			Enabled:             true,
			PollIntervalSeconds: int(e.PollInterval.Seconds()),
		}
	}
	err := engine.WithTx(ctx, func(ctx context.Context) error {
		repo := drel.NewTxRepository(drel.MustFromContext(ctx), SourceMeta)
		_, err := repo.BulkUpsert(ctx, rows,
			drel.ConflictColumns(Sources.Kind), drel.ConflictColumns(Sources.URL),
			drel.UpdateOnConflict(Sources.Title, Sources.Language), drel.UpdateOnConflict(Sources.Tier))
		return err
	})
	if err != nil {
		return fmt.Errorf("seed sources: %w", err)
	}
	return nil
}
