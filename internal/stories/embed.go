package stories

import (
	"context"
	"fmt"

	"github.com/alternayte/drel"

	"github.com/alternayte/briefing/internal/embed"
	"github.com/alternayte/briefing/internal/ingest"
)

// ItemEmbedder embeds kept items that have no vector yet.
type ItemEmbedder struct {
	engine   *drel.Engine
	embedder *embed.Embedder
}

func NewItemEmbedder(engine *drel.Engine, e *embed.Embedder) *ItemEmbedder {
	return &ItemEmbedder{engine: engine, embedder: e}
}

// Embed embeds the kept items among ids in one batch and returns the ids it
// embedded.
func (e *ItemEmbedder) Embed(ctx context.Context, ids []int64) ([]int64, error) {
	var items []*ingest.Item
	err := e.engine.WithTx(ctx, func(ctx context.Context) (err error) {
		items, err = drel.NewTxRepository(drel.MustFromContext(ctx), ingest.ItemMeta).
			Where(drel.And(ingest.Items.ID.In(ids...), ingest.Items.Status.Eq(ingest.ItemKept), ingest.Items.EmbedModel.Eq(""))).All(ctx)
		return err
	})
	if err != nil || len(items) == 0 {
		return nil, err
	}
	texts := make([]string, len(items))
	for i, it := range items {
		texts[i] = it.Title + "\n" + it.Snippet
	}
	vectors, err := e.embedder.Embed(ctx, "embed", texts)
	if err != nil {
		return nil, err
	}
	done := make([]int64, len(items))
	err = e.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		for i, it := range items {
			v, err := vectors[i].Value()
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE items SET embedding = $2::vector, embed_model = $3, updated_at = now() WHERE id = $1`,
				it.ID(), v, e.embedder.Model()); err != nil {
				return err
			}
			done[i] = it.ID()
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("store embeddings: %w", err)
	}
	return done, nil
}
