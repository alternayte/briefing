package sources

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

const atom = `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>My Blog</title>
<entry><title>Hello</title><link href="https://my.blog/hello"/><id>1</id><updated>2026-10-16T00:00:00Z</updated></entry></feed>`

func TestAddFeedRefusesAPrivateAddress(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(atom)) }))
	defer srv.Close()
	public := platform.NewPublicClient(5*time.Second, 3)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		user := pgtest.User(t, ctx, "rss-private@example.com")
		_, err := AddFeed(ctx, e, public, platform.DefaultTuning(), user, srv.URL+"/feed")
		if !errors.Is(err, ErrFeedUnreachable) || !errors.Is(err, platform.ErrPrivateAddress) {
			t.Fatalf("err = %v", err)
		}
		var n int
		_ = drel.MustFromContext(ctx).QueryRow(ctx, `SELECT count(*) FROM sources WHERE url = $1`, srv.URL+"/feed").Scan(&n)
		if n != 0 {
			t.Error("a private feed was stored")
		}
	})
}

func TestAddFeedStoresAndSubscribes(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/page" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html><body>not a feed</body></html>"))
			return
		}
		_, _ = w.Write([]byte(atom))
	}))
	defer srv.Close()
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		user := pgtest.User(t, ctx, "rss@example.com")
		l, err := AddFeed(ctx, e, srv.Client(), platform.DefaultTuning(), user, srv.URL+"/feed#x")
		if err != nil || l.Title != "My Blog" || l.Tier != 3 || !l.Subscribed {
			t.Fatalf("added %+v (%v)", l, err)
		}
		listed, err := ForReader(ctx, e, user)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, s := range listed {
			found = found || (s.ID == l.ID && s.Subscribed)
		}
		if !found {
			t.Error("the custom feed is not in the reader's list")
		}
		if _, err := AddFeed(ctx, e, srv.Client(), platform.DefaultTuning(), user, srv.URL+"/page"); !errors.Is(err, ErrNotAFeed) {
			t.Errorf("html page: %v", err)
		}
		if _, err := AddFeed(ctx, e, srv.Client(), platform.DefaultTuning(), user, "javascript:alert(1)"); !errors.Is(err, ErrFeedURL) {
			t.Errorf("bad url: %v", err)
		}
	})
}
