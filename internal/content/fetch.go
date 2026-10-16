// Package content fetches articles and extracts their text.
package content

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"

	readability "codeberg.org/readeck/go-readability/v2"

	"github.com/alternayte/briefing/internal/platform"
)

// ErrNotHTML reports a page that is not an HTML document.
var ErrNotHTML = errors.New("not an HTML page")

// Fetcher downloads pages with one request at a time per host.
type Fetcher struct {
	client    *http.Client
	maxBytes  int64
	userAgent string

	mu    sync.Mutex
	hosts map[string]*sync.Mutex
}

func NewFetcher(client *http.Client, t platform.Tuning, userAgent string) *Fetcher {
	return &Fetcher{client: client, maxBytes: t.Fetch.MaxBytes, userAgent: userAgent, hosts: map[string]*sync.Mutex{}}
}

func (f *Fetcher) hostLock(host string) *sync.Mutex {
	f.mu.Lock()
	defer f.mu.Unlock()
	l, ok := f.hosts[host]
	if !ok {
		l = &sync.Mutex{}
		f.hosts[host] = l
	}
	return l
}

// FetchText downloads a page and returns its readable text.
func (f *Fetcher) FetchText(ctx context.Context, rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	lock := f.hostLock(u.Hostname())
	lock.Lock()
	page, final, err := f.get(ctx, rawURL)
	lock.Unlock()
	if err != nil {
		return "", err
	}
	return Extract(page, final)
}

func (f *Fetcher) get(ctx context.Context, rawURL string) ([]byte, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.1")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("fetch %s: status %d", rawURL, resp.StatusCode)
	}
	mt, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mt != "text/html" && mt != "application/xhtml+xml" {
		return nil, nil, fmt.Errorf("fetch %s: %w (%s)", rawURL, ErrNotHTML, mt)
	}
	// Past the cap the rest of the page is dropped; the start of an article
	// is what the summary needs.
	page, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBytes))
	if err != nil {
		return nil, nil, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	return page, resp.Request.URL, nil
}

// Extract returns the readable text of an HTML page as paragraphs.
func Extract(page []byte, pageURL *url.URL) (string, error) {
	article, err := readability.FromReader(bytes.NewReader(page), pageURL)
	if err != nil {
		return "", fmt.Errorf("extract: %w", err)
	}
	var b strings.Builder
	if err := article.RenderText(&b); err != nil {
		return "", fmt.Errorf("extract: %w", err)
	}
	return normalize(b.String()), nil
}

// normalize trims each line and keeps at most one blank line between
// paragraphs.
func normalize(s string) string {
	var out []string
	blank := false
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			blank = len(out) > 0
			continue
		}
		if blank {
			out = append(out, "")
			blank = false
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// Renderer turns a JavaScript-built page into Markdown with Cloudflare
// Browser Run.
type Renderer struct {
	client  *http.Client
	base    string
	account string
	token   string
}

// CloudflareAPI is the base of the Cloudflare API.
const CloudflareAPI = "https://api.cloudflare.com"

func NewRenderer(client *http.Client, base, account, token string) *Renderer {
	return &Renderer{client: client, base: base, account: account, token: token}
}

// Render returns the rendered page as Markdown text.
func (r *Renderer) Render(ctx context.Context, pageURL string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"url":         pageURL,
		"gotoOptions": map[string]string{"waitUntil": "networkidle2"},
	})
	endpoint := r.base + "/client/v4/accounts/" + url.PathEscape(r.account) + "/browser-run/markdown"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("render %s: %w", pageURL, err)
	}
	defer resp.Body.Close()
	var out struct {
		Success bool   `json:"success"`
		Result  string `json:"result"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&out); err != nil {
		return "", fmt.Errorf("render %s: status %d: %w", pageURL, resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK || !out.Success {
		msg := ""
		if len(out.Errors) > 0 {
			msg = out.Errors[0].Message
		}
		return "", fmt.Errorf("render %s: status %d: %s", pageURL, resp.StatusCode, msg)
	}
	return normalize(out.Result), nil
}
