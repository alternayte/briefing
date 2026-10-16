// Package accounts wires auth-all and owns profiles, locale and access tokens.
package accounts

import (
	"fmt"
	"net/http"
	"time"

	authall "github.com/alternayte/auth-all"
	"github.com/alternayte/auth-all/oauth/github"
	"github.com/alternayte/auth-all/plugin"
	"github.com/alternayte/auth-all/plugins/apikeys"
	"github.com/alternayte/auth-all/plugins/magiclink"
	"github.com/alternayte/auth-all/plugins/roles"
	"github.com/alternayte/auth-all/ratelimit"
	"github.com/alternayte/auth-all/ratelimit/storelimit"
	"github.com/alternayte/auth-all/schema"
	"github.com/alternayte/auth-all/store/postgres"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/platform"
)

// AccessTokenPrefix starts every personal access token.
const AccessTokenPrefix = "brf_"

const (
	RoleMember = "member"
	RoleAdmin  = "admin"
)

// Auth is auth-all plus the API key plugin that issues personal access
// tokens.
type Auth struct {
	*authall.Auth
	Keys *apikeys.Plugin
}

// NewAuth builds auth-all over the shared pool.
func NewAuth(cfg platform.Config, pool *pgxpool.Pool) (*Auth, error) {
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
	keys := apikeys.New(apikeys.Prefix(AccessTokenPrefix), apikeys.AdminRole(RoleAdmin))
	plugins := []plugin.Plugin{roles.New(roles.Hierarchy(RoleMember, RoleAdmin)), keys}
	opts := []authall.Option{}
	if cfg.MagicLinkEnabled() {
		plugins = append(plugins, magiclink.New(
			// The emailed link opens the app's own localized confirmation page.
			magiclink.WithVerifyURL(cfg.BaseURL.JoinPath("/login/verify").String()),
		))
		opts = append(opts, authall.WithEmailSender(newResendSender(cfg, resendEndpoint, &http.Client{Timeout: 10 * time.Second})))
	}
	auth, err := authall.New(append(opts,
		authall.WithStore(st),
		authall.WithRateLimiter(limiter),
		authall.WithStrictRateLimiting(),
		authall.WithSchema(schema.Options{IDType: schema.IDUUID}),
		authall.WithBaseURL(cfg.BaseURL.String()),
		authall.WithProvider(github.New(
			github.WithClientID(cfg.GitHubClientID),
			github.WithClientSecret(cfg.GitHubClientSecret),
		)),
		authall.WithPlugins(plugins...),
	)...)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	applyAllowlist(auth.Hooks(), cfg)
	return &Auth{Auth: auth, Keys: keys}, nil
}

// Handler serves the auth-all routes under /api/auth/.
func Handler(auth *authall.Auth) http.Handler {
	return withRequestLocale(browserErrors(auth.Handler()))
}

// withRequestLocale puts the browser's preferred locale in the context, so
// an email sent during the request uses it.
func withRequestLocale(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := i18n.FromAcceptLanguage(r.Header.Get("Accept-Language"))
		next.ServeHTTP(w, r.WithContext(i18n.WithLocale(r.Context(), l)))
	})
}
