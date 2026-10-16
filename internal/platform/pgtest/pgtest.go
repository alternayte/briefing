// Package pgtest opens the migrated test database for DB tests.
package pgtest

import (
	"context"
	"os"
	"testing"

	"github.com/alternayte/drel"
	"github.com/google/uuid"
)

// Engine returns an engine on TEST_DATABASE_URL, or skips the test when it is
// unset. The database must be migrated; just test does that.
func Engine(t testing.TB) *drel.Engine {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	e, err := drel.NewEngine(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Close)
	return e
}

// User inserts an auth-all user inside the transaction of ctx and returns its id.
func User(t testing.TB, ctx context.Context, email string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := drel.MustFromContext(ctx).Exec(ctx, `INSERT INTO auth_users (id, email, email_normalized, display_name, image_url, created_at, updated_at)
		VALUES ($1, $2, lower($2), '', '', now(), now())`, id, email)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
