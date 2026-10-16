package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/alternayte/briefing/internal/platform"
)

// freshDatabase creates an empty database next to TEST_DATABASE_URL and drops
// it after the test, because migrations change the whole schema.
func freshDatabase(t *testing.T) string {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("briefing_migrate_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)")
		_ = admin.Close(context.Background())
	})
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}

func TestMigrateCreatesEverySchemaAndIsRepeatable(t *testing.T) {
	dsn := freshDatabase(t)
	cfg, err := platform.LoadConfig(func(k string) (string, bool) {
		v, ok := map[string]string{
			"DATABASE_URL":         dsn,
			"BASE_URL":             "http://localhost:8080",
			"SESSION_SECRET":       strings.Repeat("s", 32),
			"GITHUB_CLIENT_ID":     "id",
			"GITHUB_CLIENT_SECRET": "secret",
			"OPENROUTER_API_KEY":   "key",
		}[k]
		return v, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for range 2 {
		if err := migrate(ctx, cfg); err != nil {
			t.Fatal(err)
		}
	}

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	for _, table := range []string{"auth_users", "auth_api_keys", "auth_rate_limits", "river_job", "drel_migrations"} {
		var ok bool
		if err := conn.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&ok); err != nil || !ok {
			t.Errorf("table %s missing (err %v)", table, err)
		}
	}
	var vector bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector')").Scan(&vector); err != nil || !vector {
		t.Errorf("extension vector missing (err %v)", err)
	}
}
