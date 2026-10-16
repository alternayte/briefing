package platform

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/alternayte/drel"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// OpenPool connects to Postgres and checks the connection.
func OpenPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// MigrateRiver applies the River job tables.
func MigrateRiver(ctx context.Context, pool *pgxpool.Pool) error {
	m, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("river migrator: %w", err)
	}
	if _, err := m.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("river migrate: %w", err)
	}
	return nil
}

// MigrateApp applies the app migration sets in version order.
func MigrateApp(ctx context.Context, dsn string, sets ...fs.FS) error {
	engine, err := drel.NewEngine(dsn)
	if err != nil {
		return fmt.Errorf("open drel engine: %w", err)
	}
	defer engine.Close()
	if _, err := engine.ApplyMigrationsFS(ctx, sets...); err != nil {
		return fmt.Errorf("app migrate: %w", err)
	}
	return nil
}
