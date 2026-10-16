package content

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alternayte/briefing/internal/platform"
)

func articleServer(t *testing.T) *httptest.Server {
	t.Helper()
	page, err := os.ReadFile("testdata/article.html")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pdf":
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.7"))
		case "/huge":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html><body><p>" + strings.Repeat("word ", 1<<20) + "</p></body></html>"))
		case "/gone":
			http.NotFound(w, r)
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(page)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestExtractKeepsTheArticleAndDropsTheChrome(t *testing.T) {
	srv := articleServer(t)
	f := NewFetcher(srv.Client(), platform.DefaultTuning(), "test")
	text, err := f.FetchText(context.Background(), srv.URL+"/story")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"touched down on the comet", "heavy to normal water"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q", want)
		}
	}
	for _, unwanted := range []string{"Subscribe now", "premium plan", "Ten gadgets", "Privacy"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("text keeps %q", unwanted)
		}
	}
}

func TestFetchRefusesNonHTMLAndErrors(t *testing.T) {
	srv := articleServer(t)
	f := NewFetcher(srv.Client(), platform.DefaultTuning(), "test")
	if _, err := f.FetchText(context.Background(), srv.URL+"/pdf"); !errors.Is(err, ErrNotHTML) {
		t.Errorf("pdf: %v", err)
	}
	if _, err := f.FetchText(context.Background(), srv.URL+"/gone"); err == nil {
		t.Error("404 accepted")
	}
}

func TestFetchStopsAtTheSizeCap(t *testing.T) {
	srv := articleServer(t)
	tu := platform.DefaultTuning()
	f := NewFetcher(srv.Client(), tu, "test")
	text, err := f.FetchText(context.Background(), srv.URL+"/huge")
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(text)) > tu.Fetch.MaxBytes {
		t.Errorf("read %d bytes past the %d cap", len(text), tu.Fetch.MaxBytes)
	}
}

func TestOneRequestAtATimePerHost(t *testing.T) {
	var inFlight, peak atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := inFlight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		inFlight.Add(-1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body><p>x</p></body></html>"))
	}))
	defer srv.Close()
	f := NewFetcher(srv.Client(), platform.DefaultTuning(), "test")
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = f.FetchText(context.Background(), srv.URL+"/") }()
	}
	wg.Wait()
	if peak.Load() != 1 {
		t.Errorf("%d requests to one host at once", peak.Load())
	}
}

func TestRendererReturnsMarkdownText(t *testing.T) {
	var auth, sentURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if !strings.HasSuffix(r.URL.Path, "/accounts/acct/browser-run/markdown") {
			http.NotFound(w, r)
			return
		}
		var body struct{ URL string }
		_ = jsonDecode(r, &body)
		sentURL = body.URL
		_, _ = w.Write([]byte(`{"success":true,"result":"# Probe lands\n\nThe probe touched down."}`))
	}))
	defer srv.Close()
	r := NewRenderer(srv.Client(), srv.URL, "acct", "tok")
	text, err := r.Render(context.Background(), "https://example.com/a")
	if err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer tok" || sentURL != "https://example.com/a" || !strings.Contains(text, "touched down") {
		t.Errorf("auth=%q url=%q text=%q", auth, sentURL, text)
	}
}

func TestRendererFailureIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":2001,"message":"bad url"}]}`))
	}))
	defer srv.Close()
	if _, err := NewRenderer(srv.Client(), srv.URL, "acct", "tok").Render(context.Background(), "x"); err == nil {
		t.Error("want an error")
	}
}

// Publishers answer bursts with 429, so requests to one host keep a gap.
func TestRequestsToOneHostKeepTheGap(t *testing.T) {
	var mu sync.Mutex
	var at []time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		at = append(at, time.Now())
		mu.Unlock()
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body><p>x</p></body></html>"))
	}))
	defer srv.Close()
	tun := platform.DefaultTuning()
	tun.Fetch.HostGap = 80 * time.Millisecond
	f := NewFetcher(srv.Client(), tun, "test")
	for range 3 {
		_, _ = f.FetchText(context.Background(), srv.URL+"/")
	}
	for i := 1; i < len(at); i++ {
		if d := at[i].Sub(at[i-1]); d < tun.Fetch.HostGap {
			t.Errorf("request %d came %v after the previous one", i, d)
		}
	}
}
