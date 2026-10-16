package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/alternayte/drel"

	"github.com/alternayte/briefing/internal/sources"
)

// Poller polls one source and stores what it links to.
type Poller struct {
	engine *drel.Engine
	conn   connectors
	now    func() time.Time
}

func NewPoller(engine *drel.Engine, conn connectors, now func() time.Time) *Poller {
	return &Poller{engine: engine, conn: conn, now: now}
}

// Poll fetches a source, stores its items and sightings, and saves the
// checkpoint in the same transaction, so a crash never skips an item. It
// returns the ids of the items the poll touched.
func (p *Poller) Poll(ctx context.Context, sourceID int64) ([]int64, error) {
	src, err := sources.Find(ctx, p.engine, sourceID)
	if errors.Is(err, drel.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !src.Enabled {
		return nil, nil
	}
	res, err := p.conn.fetch(ctx, src)
	if err != nil {
		if recErr := sources.RecordError(ctx, p.engine, src.ID(), err); recErr != nil {
			slog.ErrorContext(ctx, "record poll error", "source", src.ID(), "error", recErr)
		}
		return nil, fmt.Errorf("poll source %d: %w", src.ID(), err)
	}
	var ids []int64
	err = p.engine.WithTx(ctx, func(ctx context.Context) error {
		ids, err = p.store(ctx, src, res)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("store poll of source %d: %w", src.ID(), err)
	}
	return ids, nil
}

func (p *Poller) store(ctx context.Context, src *sources.Source, res Result) ([]int64, error) {
	tx := drel.MustFromContext(ctx)
	now := p.now().UTC()

	byURL := map[string]Entry{}
	var items []*Item
	for _, e := range res.Entries {
		canonical, err := CanonicalURL(e.URL)
		if err != nil {
			continue
		}
		if _, dup := byURL[canonical]; dup {
			continue
		}
		byURL[canonical] = e
		it := &Item{CanonicalURL: canonical, Title: e.Title, Snippet: e.Snippet, Status: ItemNew, TextSource: TextPending}
		if !e.PublishedAt.IsZero() {
			published := e.PublishedAt
			it.PublishedAt = &published
		}
		items = append(items, it)
	}

	var ids []int64
	if len(items) > 0 {
		itemRepo := drel.NewTxRepository(tx, ItemMeta)
		if _, err := itemRepo.BulkUpsert(ctx, items, drel.ConflictColumns(Items.CanonicalURL), drel.DoNothing()); err != nil {
			return nil, err
		}
		urls := make([]string, 0, len(byURL))
		for u := range byURL {
			urls = append(urls, u)
		}
		stored, err := itemRepo.Where(Items.CanonicalURL.In(urls...)).All(ctx)
		if err != nil {
			return nil, err
		}
		sightings := make([]*Sighting, 0, len(stored))
		for _, it := range stored {
			e := byURL[it.CanonicalURL]
			ids = append(ids, it.ID())
			sightings = append(sightings, &Sighting{SourceID: src.ID(), ExternalID: e.ExternalID, ItemID: it.ID(), Engagement: e.Engagement, SeenAt: now})
		}
		_, err = drel.NewTxRepository(tx, SightingMeta).BulkUpsert(ctx, sightings,
			drel.ConflictColumns(Sightings.SourceID), drel.ConflictColumns(Sightings.ExternalID),
			drel.UpdateOnConflict(Sightings.Engagement), drel.UpdateOnConflict(Sightings.SeenAt))
		if err != nil {
			return nil, err
		}
	}

	return ids, sources.SaveCheckpoint(ctx, src.ID(), res.Checkpoint.ETag, res.Checkpoint.LastModified, now)
}
