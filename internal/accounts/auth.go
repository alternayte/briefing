// Package accounts wires auth-all and owns profiles, locale and access tokens.
package accounts

import (
	"fmt"

	authall "github.com/alternayte/auth-all"
	"github.com/alternayte/auth-all/oauth/github"
	"github.com/alternayte/auth-all/plugins/apikeys"
	"github.com/alternayte/auth-all/plugins/roles"
	"github.com/alternayte/auth-all/ratelimit"
	"github.com/alternayte/auth-all/ratelimit/storelimit"
	"github.com/alternayte/auth-all/schema"
	"github.com/alternayte/auth-all/store/postgres"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alternayte/briefing/internal/platform"
)

// AccessTokenPrefix starts every personal access token.
const AccessTokenPrefix = "brf_"

const (
	RoleMember = "member"
	RoleAdmin  = "admin"
)

// NewAuth builds auth-all over the shared pool.
func NewAuth(cfg platform.Config, pool *pgxpool.Pool) (*authall.Auth, error) {
	// No transaction pooler sits in front of Postgres, so cached statements
	// are safe.
	st, err := postgres.NewPool(pool, postgres.AllowStatementCache())
	if err != nil {
		return nil, fmt.Errorf("auth store: %w", err)
	}
	limiter, err := storelimit.New(st, ratelimit.DefaultSignInRules())
	if err != nil {
		return nil, fmt.Errorf("auth rate limiter: %w", err)
	}
	auth, err := authall.New(
		authall.WithStore(st),
		authall.WithRateLimiter(limiter),
		authall.WithStrictRateLimiting(),
		authall.WithSchema(schema.Options{IDType: schema.IDUUID}),
		authall.WithBaseURL(cfg.BaseURL.String()),
		authall.WithProvider(github.New(
			github.WithClientID(cfg.GitHubClientID),
			github.WithClientSecret(cfg.GitHubClientSecret),
		)),
		authall.WithPlugins(
			roles.New(roles.Hierarchy(RoleMember, RoleAdmin)),
			apikeys.New(apikeys.Prefix(AccessTokenPrefix), apikeys.AdminRole(RoleAdmin)),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	return auth, nil
}
