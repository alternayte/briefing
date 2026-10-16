package main

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/alternayte/gx"
	"github.com/alternayte/gx/adapters/datastar"
	"github.com/alternayte/gx/gxmcp"

	"github.com/alternayte/briefing/app"
	"github.com/alternayte/briefing/gxstyles"
	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/admin"
	"github.com/alternayte/briefing/internal/feed"
	"github.com/alternayte/briefing/internal/learning"
	"github.com/alternayte/briefing/internal/sources"
)

// newHandler mounts auth-all, the health check and the Gx app on one mux.
func newHandler(web *accounts.Web, feedWeb *feed.Web, sourcesWeb *sources.Web, learningWeb *learning.Web, adminWeb *admin.Web, ping func(context.Context) error) http.Handler {
	setupGallery()
	gx.SetStylesheet(gxstyles.CSS())
	gx.SetWidgetStylesheets(gxstyles.Widgets())
	gx.SetRouteStylesheets(gxstyles.Routes())
	ui := gx.New(gx.Config{Adapter: datastar.Adapter()})
	ui.Group("/", app.Layout, gx.Nav(gx.MorphNavigation), accounts.Routes)
	ui.Group("/", app.Layout, gx.Nav(gx.MorphNavigation), feedWeb.Middleware, feed.Routes)
	ui.Group("/", app.Layout, gx.Nav(gx.MorphNavigation), sourcesWeb.Middleware, sources.Routes)
	ui.Group("/", app.Layout, gx.Nav(gx.MorphNavigation), learningWeb.Middleware, learning.Routes)
	ui.Group("/", app.Layout, gx.Nav(gx.MorphNavigation), adminWeb.Middleware, admin.Routes)
	gxmcp.Mount(ui, "/mcp", accounts.RequireAccessToken)

	mux := http.NewServeMux()
	mux.Handle("GET /healthz", healthz(ping))
	mux.Handle("GET /llms.txt", llmsTxt(web.BaseURL()))
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

// llmsTxt describes Briefing to language models, at the site root.
func llmsTxt(baseURL string) http.Handler {
	base := strings.TrimSuffix(baseURL, "/")
	body := `# Briefing

> Briefing is a learning-first news brief. It reads tech and science sources, drops the noise, merges coverage of one event into one story, and shows each reader a short ranked feed of source-backed summaries. It then helps the reader keep what they read, with explanations at their level, spaced review questions and a record of the concepts they learned. It is in English and French.

## Use it from an AI assistant

- MCP server (streamable HTTP, stateless): ` + base + `/mcp
- Authentication: a personal access token as a bearer token, ` + "`Authorization: Bearer brf_…`" + `. The reader creates and revokes tokens in Settings: ` + base + `/settings
- Agent Skill with the workflows (morning brief, digging into a story, review session, tuning interests): https://github.com/alternayte/briefing/tree/main/skills/briefing

## Tools

- feed_today, feed_search, feed_story, feed_explain, feed_feedback
- learning_review_due, learning_review_answer, learning_learned
- feed_interests_list, feed_interests_add, feed_interests_remove
- sources_list, sources_subscribe, sources_add_rss
- accounts_me

## Source

- https://github.com/alternayte/briefing
`
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write([]byte(body))
	})
}
