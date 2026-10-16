package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/alternayte/drel"

	"github.com/alternayte/briefing/catalog"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/sources"
)

// seed loads the embedded source catalog.
func seed(ctx context.Context, cfg platform.Config) error {
	entries, err := sources.ParseCatalog(catalog.Sources, platform.DefaultTuning())
	if err != nil {
		return err
	}
	engine, err := drel.NewEngine(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open drel engine: %w", err)
	}
	defer engine.Close()
	if err := sources.Seed(ctx, engine, entries); err != nil {
		return err
	}
	slog.Info("seeded sources", "count", len(entries))
	return nil
}
