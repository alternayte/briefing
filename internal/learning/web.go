package learning

import (
	"context"
	"net/http"
	"time"

	"github.com/alternayte/drel"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/platform"
)

// Web holds what the learning pages need at request time.
type Web struct {
	engine *drel.Engine
	tuning platform.Tuning
	now    func() time.Time
}

func NewWeb(engine *drel.Engine, t platform.Tuning, now func() time.Time) *Web {
	return &Web{engine: engine, tuning: t, now: now}
}

type webKey struct{}

func webFrom(ctx context.Context) *Web { return ctx.Value(webKey{}).(*Web) }

// Middleware sends signed-out visitors to the sign-in page.
func (w *Web) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if accounts.CurrentProfile(r.Context()) == nil {
			http.Redirect(rw, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(rw, r.WithContext(context.WithValue(r.Context(), webKey{}, w)))
	})
}
