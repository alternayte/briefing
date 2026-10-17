package feed

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/alternayte/drel"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/embed"
	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/translate"
	"github.com/alternayte/briefing/internal/write"
)

// Web holds what the feed pages need at request time.
type Web struct {
	engine     *drel.Engine
	embedder   *embed.Embedder
	explainer  *write.Explainer
	translator *translate.Translator
	profiles   *accounts.ProfileStore
	jobs       *river.Client[pgx.Tx]
	tuning     platform.Tuning
	baseURL    string
	now        func() time.Time
}

func NewWeb(engine *drel.Engine, e *embed.Embedder, x *write.Explainer, tr *translate.Translator, profiles *accounts.ProfileStore, jobs *river.Client[pgx.Tx], t platform.Tuning, baseURL string, now func() time.Time) *Web {
	return &Web{engine: engine, embedder: e, explainer: x, translator: tr, profiles: profiles, jobs: jobs, tuning: t, baseURL: strings.TrimSuffix(baseURL, "/"), now: now}
}

type webKey struct{}

func webFrom(ctx context.Context) *Web { return ctx.Value(webKey{}).(*Web) }

// viewOf returns how the reader of the request sees cards now.
func (w *Web) viewOf(ctx context.Context, profile *accounts.Profile) view {
	return view{locale: i18n.FromContext(ctx), loc: profile.Location(), now: w.now().UTC()}
}

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
