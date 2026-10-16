package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"

	"github.com/alternayte/briefing/app"
	"github.com/alternayte/briefing/gxstyles"
	"github.com/alternayte/briefing/internal/accounts"
)

// newHandler mounts auth-all, the health check and the Gx app on one mux.
func newHandler(web *accounts.Web, ping func(context.Context) error) http.Handler {
	setupGallery()
	gx.SetStylesheet(gxstyles.CSS())
	gx.SetWidgetStylesheets(gxstyles.Widgets())
	gx.SetRouteStylesheets(gxstyles.Routes())
	ui := gx.New(gx.Config{Adapter: datastar.Adapter()})
	ui.Group("/", app.Layout, gx.Nav(gx.MorphNavigation), accounts.Routes)

	mux := http.NewServeMux()
	mux.Handle("GET /healthz", healthz(ping))
	mux.Handle("/api/auth/", web.AuthHandler())
	mux.Handle("/", web.Middleware(ui))
	// Datastar evaluates client expressions at run time, so the policy needs
	// 'unsafe-eval'.
	return gx.CSP(gx.CSPOptions{
		UnsafeEval: true,
		Directives: "img-src 'self' data: https:; frame-ancestors 'none'; form-action 'self' https://github.com",
	})(mux)
}

// healthz answers 200 when the database answers within two seconds.
func healthz(ping func(context.Context) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		w.Header().Set("Cache-Control", "no-store")
		if err := ping(ctx); err != nil {
			slog.WarnContext(ctx, "health check failed", "error", err)
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok\n"))
	})
}
