package accounts

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	authall "github.com/alternayte/auth-all"
	"github.com/alternayte/drel"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/platform"
)

// Web holds what the accounts pages need at request time. Gx pages are
// package values, so Web travels in the request context.
type Web struct {
	cfg      platform.Config
	auth     *Auth
	authHTTP http.Handler
	profiles *ProfileStore
}

func NewWeb(cfg platform.Config, auth *Auth, profiles *ProfileStore) *Web {
	return &Web{cfg: cfg, auth: auth, authHTTP: Handler(auth.Auth), profiles: profiles}
}

// RequireAccessToken lets only requests that carry a personal access token
// pass. It runs after Middleware.
func RequireAccessToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := authall.PrincipalFrom(r.Context()); p == nil || p.Method != authall.MethodAPIKey {
			w.Header().Set("WWW-Authenticate", `Bearer realm="briefing"`)
			http.Error(w, "a personal access token is required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AuthHandler serves /api/auth/.
func (w *Web) AuthHandler() http.Handler { return w.authHTTP }

type webKey struct{}
type profileKey struct{}

func webFrom(ctx context.Context) *Web { return ctx.Value(webKey{}).(*Web) }

// WithProfile returns a context whose signed-in user has profile p.
func WithProfile(ctx context.Context, p *Profile) context.Context {
	return context.WithValue(ctx, profileKey{}, p)
}

// CurrentProfile returns the profile of the signed-in user, or nil.
func CurrentProfile(ctx context.Context) *Profile {
	p, _ := ctx.Value(profileKey{}).(*Profile)
	return p
}

// Middleware loads the session, creates the profile on first use, and sets
// the request locale: the profile's when signed in, else the browser's.
func (w *Web) Middleware(next http.Handler) http.Handler {
	return w.auth.LoadSession(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), webKey{}, w)
		locale := i18n.FromAcceptLanguage(r.Header.Get("Accept-Language"))
		if u := authall.UserFrom(ctx); u != nil {
			p, err := w.ensureProfile(ctx, u.ID, u.DisplayName, locale)
			if err != nil {
				slog.ErrorContext(ctx, "load profile", "user", u.ID, "error", err)
				http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			ctx = WithProfile(ctx, p)
			locale = p.UserLocale()
		}
		next.ServeHTTP(rw, r.WithContext(i18n.WithLocale(ctx, locale)))
	}))
}

func (w *Web) ensureProfile(ctx context.Context, userID, displayName string, locale i18n.Locale) (*Profile, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	p, err := w.profiles.Get(ctx, id)
	if errors.Is(err, drel.ErrNotFound) {
		return w.profiles.Ensure(ctx, id, displayName, locale)
	}
	return p, err
}

// IsAdmin reports whether the signed-in user is listed in ADMIN_EMAILS.
func IsAdmin(ctx context.Context) bool {
	u := authall.UserFrom(ctx)
	w, ok := ctx.Value(webKey{}).(*Web)
	return u != nil && ok && w.cfg.IsAdmin(u.Email)
}
