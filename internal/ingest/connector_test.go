package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/sources"
)

func serveFixture(t *testing.T, path, contentType string, handle func(w http.ResponseWriter, r *http.Request) bool) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handle != nil && handle(w, r) {
			return
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testConnectors(srv *httptest.Server) connectors {
	return newConnectors(srv.Client(), platform.DefaultTuning(), "Briefing-test", srv.URL)
}

func TestRSSAndAtomFeedsYieldEntries(t *testing.T) {
	for _, fx := range []struct{ file, ctype string }{
		{"testdata/goblog.atom", "application/atom+xml"},
		{"testdata/krebs.rss", "application/rss+xml"},
	} {
		srv := serveFixture(t, fx.file, fx.ctype, nil)
		src := &sources.Source{Kind: sources.KindRSS, URL: srv.URL + "/feed"}
		res, err := testConnectors(srv).fetch(context.Background(), src)
		if err != nil {
			t.Fatalf("%s: %v", fx.file, err)
		}
		if len(res.Entries) != 3 {
			t.Fatalf("%s: %d entries", fx.file, len(res.Entries))
		}
		for _, e := range res.Entries {
			if e.Title == "" || !strings.HasPrefix(e.URL, "https://") || e.ExternalID == "" || e.PublishedAt.IsZero() {
				t.Errorf("%s: incomplete entry %+v", fx.file, e)
			}
			if strings.Contains(e.Snippet, "<") || len([]rune(e.Snippet)) > platform.DefaultTuning().Poll.SnippetChars {
				t.Errorf("%s: snippet not plain or too long: %q", fx.file, e.Snippet)
			}
		}
	}
}

func TestRSSConditionalGet(t *testing.T) {
	srv := serveFixture(t, "testdata/krebs.rss", "application/rss+xml", func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return true
		}
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Last-Modified", "Fri, 16 Oct 2026 01:08:46 GMT")
		return false
	})
	c := testConnectors(srv)
	src := &sources.Source{Kind: sources.KindRSS, URL: srv.URL + "/feed"}
	first, err := c.fetch(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if first.NotModified || first.Checkpoint.ETag != `"v1"` || first.Checkpoint.LastModified == "" {
		t.Fatalf("first poll = %+v", first.Checkpoint)
	}
	src.ETag, src.LastModified = first.Checkpoint.ETag, first.Checkpoint.LastModified
	second, err := c.fetch(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	if !second.NotModified || len(second.Entries) != 0 {
		t.Errorf("second poll = %+v", second)
	}
}

func TestFeedErrorsAreErrors(t *testing.T) {
	srv := serveFixture(t, "testdata/krebs.rss", "application/rss+xml", func(w http.ResponseWriter, r *http.Request) bool {
		http.Error(w, "gone", http.StatusGone)
		return true
	})
	_, err := testConnectors(srv).fetch(context.Background(), &sources.Source{Kind: sources.KindRSS, URL: srv.URL})
	if err == nil {
		t.Error("410 accepted")
	}
}

func TestHackerNewsFrontPage(t *testing.T) {
	srv := serveFixture(t, "testdata/hn.json", "application/json", nil)
	res, err := testConnectors(srv).fetch(context.Background(), &sources.Source{Kind: sources.KindHN, URL: srv.URL + "/api/v1/search?tags=front_page"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) == 0 {
		t.Fatal("no entries")
	}
	for _, e := range res.Entries {
		if e.URL == "" || e.Title == "" || e.ExternalID == "" || e.Engagement.Points == 0 {
			t.Errorf("incomplete entry %+v", e)
		}
	}
}

func TestBlueskyAuthorFeedKeepsLinkPostsOnly(t *testing.T) {
	var gotActor string
	srv := serveFixture(t, "testdata/bluesky.json", "application/json", func(w http.ResponseWriter, r *http.Request) bool {
		gotActor = r.URL.Query().Get("actor")
		return false
	})
	res, err := testConnectors(srv).fetch(context.Background(), &sources.Source{Kind: sources.KindBluesky, URL: "simonwillison.net"})
	if err != nil {
		t.Fatal(err)
	}
	if gotActor != "simonwillison.net" {
		t.Errorf("actor = %q", gotActor)
	}
	if len(res.Entries) != 7 {
		t.Fatalf("%d entries, want the 7 posts with a link card", len(res.Entries))
	}
	for _, e := range res.Entries {
		if !strings.HasPrefix(e.ExternalID, "at://") || e.URL == "" || e.PublishedAt.IsZero() {
			t.Errorf("incomplete entry %+v", e)
		}
	}
}
