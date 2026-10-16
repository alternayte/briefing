package feed

import (
	"context"
	"net/http"
	"time"

	"github.com/alternayte/drel"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/embed"
	"github.com/alternayte/briefing/internal/platform"
)

// Web holds what the feed pages need at request time.
type Web struct {
	engine   *drel.Engine
	embedder *embed.Embedder
	profiles *accounts.ProfileStore
	jobs     *river.Client[pgx.Tx]
	tuning   platform.Tuning
	now      func() time.Time
}

func NewWeb(engine *drel.Engine, e *embed.Embedder, profiles *accounts.ProfileStore, jobs *river.Client[pgx.Tx], t platform.Tuning, now func() time.Time) *Web {
	return &Web{engine: engine, embedder: e, profiles: profiles, jobs: jobs, tuning: t, now: now}
}

type webKey struct{}

func webFrom(ctx context.Context) *Web { return ctx.Value(webKey{}).(*Web) }

// Middleware sends signed-out visitors to the sign-in page and puts Web in
// the request context.
func (w *Web) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if accounts.CurrentProfile(r.Context()) == nil {
			http.Redirect(rw, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(rw, r.WithContext(context.WithValue(r.Context(), webKey{}, w)))
	})
}
