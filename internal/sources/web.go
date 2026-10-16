package sources

import (
	"context"
	"errors"
	"net/http"

	"github.com/alternayte/drel"
	"github.com/alternayte/gx"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/sources/route"
)

// Web holds what the source actions need at request time.
type Web struct {
	engine *drel.Engine
	client *http.Client
	tuning platform.Tuning
}

func NewWeb(engine *drel.Engine, client *http.Client, t platform.Tuning) *Web {
	return &Web{engine: engine, client: client, tuning: t}
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

// settings is the settings page, with an optional notice code.
type settings struct{ feed string }

func (s settings) URL() string {
	if s.feed == "" {
		return "/settings#sources"
	}
	return "/settings?feed=" + s.feed + "#sources"
}

// Lists the sources the reader can follow: the catalog of tech and science
// feeds, Hacker News, Bluesky accounts, and feeds the reader added. Each has
// an id, kind, title, url and whether the reader follows it. Use it before
// sources_subscribe, or when the reader asks where their stories come from.
var listSources = gx.Action(func(c *gx.Ctx, _ route.List) error {
	ctx := c.R.Context()
	list, err := ForReader(ctx, webFrom(ctx).engine, accounts.CurrentProfile(ctx).ID())
	if err != nil {
		return err
	}
	gx.ToolResult(c, list)
	return nil
}).Tool()

// Follows (on true) or unfollows (on false) one source by its id from
// sources_list. Stories that only unfollowed sources report leave the
// reader's feed from the next ranking. Returns the source.
var Subscribe = gx.Form(func(c *gx.Ctx, in *route.Subscribe) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	if err := SetSubscribed(ctx, w.engine, profile.ID(), in.SourceID, in.On); err != nil {
		return err
	}
	gx.ToolResult(c, map[string]any{"id": in.SourceID, "subscribed": in.On})
	return c.Redirect(settings{})
}, SubscribeButton).Tool()

// Adds an RSS or Atom feed by its http or https address and follows it. The
// address must be on the public internet and return a feed. Returns the new
// source, or an error that says why the feed was refused.
var AddRSS = gx.Form(func(c *gx.Ctx, in *route.AddRSS) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	l, err := AddFeed(ctx, w.engine, w.client, w.tuning, profile.ID(), in.Address)
	code := "added"
	switch {
	case errors.Is(err, ErrFeedURL):
		code = "badurl"
	case errors.Is(err, ErrNotAFeed):
		code = "notfeed"
	case errors.Is(err, ErrFeedUnreachable):
		code = "failed"
	case err != nil:
		return err
	}
	if err != nil {
		gx.ToolResult(c, map[string]string{"error": err.Error()})
	} else {
		gx.ToolResult(c, l)
	}
	return c.Redirect(settings{feed: code})
}, AddFeedForm).Tool()

// Routes lists every action and form of the slice.
var Routes = gx.Collect(listSources, Subscribe, AddRSS)
