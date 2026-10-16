package main

import (
	"context"
	"fmt"

	"github.com/alternayte/briefing/internal/accounts"
	accountsmigrations "github.com/alternayte/briefing/internal/accounts/migrations"
	ingestmigrations "github.com/alternayte/briefing/internal/ingest/migrations"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/migrations"
	sourcesmigrations "github.com/alternayte/briefing/internal/sources/migrations"
)

// migrate applies the auth-all, River and app schemas, in that order.
func migrate(ctx context.Context, cfg platform.Config) error {
	pool, err := platform.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	auth, err := accounts.NewAuth(cfg, pool)
	if err != nil {
		return err
	}
	if _, err := auth.Migrate(ctx); err != nil {
		return fmt.Errorf("auth migrate: %w", err)
	}
	if err := platform.MigrateRiver(ctx, pool); err != nil {
		return err
	}
	return platform.MigrateApp(ctx, cfg.DatabaseURL, migrations.FS, accountsmigrations.FS, sourcesmigrations.FS, ingestmigrations.FS)
}
