package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/sources"
)

const blueskyAPI = "https://public.api.bsky.app"

// Entry is one link that a source reported.
type Entry struct {
	ExternalID  string
	URL         string
	Title       string
	Snippet     string
	PublishedAt time.Time
	Engagement  Engagement
}

// Engagement is what a discovery source says about a link's reception.
type Engagement struct {
	Points   int `json:"points,omitempty"`
	Comments int `json:"comments,omitempty"`
	Likes    int `json:"likes,omitempty"`
	Reposts  int `json:"reposts,omitempty"`
	Replies  int `json:"replies,omitempty"`
}

// Checkpoint is the conditional-GET state a source keeps between polls.
type Checkpoint struct {
	ETag         string
	LastModified string
}

// Result is one poll. NotModified means the source answered 304.
type Result struct {
	Entries     []Entry
	Checkpoint  Checkpoint
	NotModified bool
}

// connectors fetches each source kind over one HTTP client.
type connectors struct {
	client     *http.Client
	tuning     platform.PollTuning
	userAgent  string
	blueskyAPI string
}

func newConnectors(client *http.Client, t platform.Tuning, userAgent, blueskyBase string) connectors {
	return connectors{client: client, tuning: t.Poll, userAgent: userAgent, blueskyAPI: blueskyBase}
}

func (c connectors) fetch(ctx context.Context, src *sources.Source) (Result, error) {
	switch src.Kind {
	case sources.KindRSS:
		return c.rss(ctx, src)
	case sources.KindHN:
		return c.hackerNews(ctx, src)
	case sources.KindBluesky:
		return c.bluesky(ctx, src)
	}
	return Result{}, fmt.Errorf("no connector for kind %q", src.Kind)
}

func (c connectors) get(ctx context.Context, rawURL string, header http.Header) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return resp, nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return resp, nil, fmt.Errorf("GET %s: status %d", rawURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, c.tuning.MaxBytes+1))
	if err != nil {
		return resp, nil, fmt.Errorf("GET %s: %w", rawURL, err)
	}
	if int64(len(body)) > c.tuning.MaxBytes {
		return resp, nil, fmt.Errorf("GET %s: body exceeds %d bytes", rawURL, c.tuning.MaxBytes)
	}
	return resp, body, nil
}

func (c connectors) rss(ctx context.Context, src *sources.Source) (Result, error) {
	h := http.Header{"Accept": {"application/rss+xml, application/atom+xml, application/xml;q=0.9, */*;q=0.5"}}
	if src.ETag != "" {
		h.Set("If-None-Match", src.ETag)
	}
	if src.LastModified != "" {
		h.Set("If-Modified-Since", src.LastModified)
	}
	resp, body, err := c.get(ctx, src.URL, h)
	if err != nil {
		return Result{}, err
	}
	cp := Checkpoint{ETag: src.ETag, LastModified: src.LastModified}
	if resp.StatusCode == http.StatusNotModified {
		return Result{Checkpoint: cp, NotModified: true}, nil
	}
	cp = Checkpoint{ETag: resp.Header.Get("ETag"), LastModified: resp.Header.Get("Last-Modified")}
	feed, err := gofeed.NewParser().ParseString(string(body))
	if err != nil {
		return Result{}, fmt.Errorf("parse feed %s: %w", src.URL, err)
	}
	base, _ := url.Parse(src.URL)
	var out []Entry
	for _, it := range feed.Items {
		link := it.Link
		if ref, err := url.Parse(link); err == nil && base != nil {
			link = base.ResolveReference(ref).String()
		}
		id := it.GUID
		if id == "" {
			id = link
		}
		published := it.PublishedParsed
		if published == nil {
			published = it.UpdatedParsed
		}
		e := Entry{
			ExternalID: id,
			URL:        link,
			Title:      plainText(it.Title, 300),
			Snippet:    plainText(firstNonEmpty(it.Description, it.Content), c.tuning.SnippetChars),
		}
		if published != nil {
			e.PublishedAt = published.UTC()
		}
		out = append(out, e)
	}
	return Result{Entries: out, Checkpoint: cp}, nil
}

type hnSearch struct {
	Hits []struct {
		ObjectID    string `json:"objectID"`
		Title       string `json:"title"`
		URL         string `json:"url"`
		Points      int    `json:"points"`
		NumComments int    `json:"num_comments"`
		CreatedAtI  int64  `json:"created_at_i"`
	} `json:"hits"`
}

func (c connectors) hackerNews(ctx context.Context, src *sources.Source) (Result, error) {
	_, body, err := c.get(ctx, src.URL, http.Header{"Accept": {"application/json"}})
	if err != nil {
		return Result{}, err
	}
	var page hnSearch
	if err := json.Unmarshal(body, &page); err != nil {
		return Result{}, fmt.Errorf("decode hn: %w", err)
	}
	var out []Entry
	for _, h := range page.Hits {
		// Ask HN and Show HN text posts link to no article.
		if h.URL == "" {
			continue
		}
		out = append(out, Entry{
			ExternalID:  h.ObjectID,
			URL:         h.URL,
			Title:       plainText(h.Title, 300),
			PublishedAt: time.Unix(h.CreatedAtI, 0).UTC(),
			Engagement:  Engagement{Points: h.Points, Comments: h.NumComments},
		})
	}
	return Result{Entries: out}, nil
}

type authorFeed struct {
	Feed []struct {
		Post struct {
			URI    string `json:"uri"`
			Record struct {
				CreatedAt time.Time `json:"createdAt"`
			} `json:"record"`
			Embed *struct {
				Type     string `json:"$type"`
				External *struct {
					URI         string `json:"uri"`
					Title       string `json:"title"`
					Description string `json:"description"`
				} `json:"external"`
			} `json:"embed"`
			LikeCount   int `json:"likeCount"`
			RepostCount int `json:"repostCount"`
			ReplyCount  int `json:"replyCount"`
		} `json:"post"`
	} `json:"feed"`
}

func (c connectors) bluesky(ctx context.Context, src *sources.Source) (Result, error) {
	q := url.Values{"actor": {src.URL}, "filter": {"posts_no_replies"}, "limit": {"50"}}
	_, body, err := c.get(ctx, c.blueskyAPI+"/xrpc/app.bsky.feed.getAuthorFeed?"+q.Encode(), http.Header{"Accept": {"application/json"}})
	if err != nil {
		return Result{}, err
	}
	var feed authorFeed
	if err := json.Unmarshal(body, &feed); err != nil {
		return Result{}, fmt.Errorf("decode bluesky: %w", err)
	}
	var out []Entry
	for _, f := range feed.Feed {
		p := f.Post
		if p.Embed == nil || p.Embed.Type != "app.bsky.embed.external#view" || p.Embed.External == nil || p.Embed.External.URI == "" {
			continue
		}
		out = append(out, Entry{
			ExternalID:  p.URI,
			URL:         p.Embed.External.URI,
			Title:       plainText(p.Embed.External.Title, 300),
			Snippet:     plainText(p.Embed.External.Description, c.tuning.SnippetChars),
			PublishedAt: p.Record.CreatedAt.UTC(),
			Engagement:  Engagement{Likes: p.LikeCount, Reposts: p.RepostCount, Replies: p.ReplyCount},
		})
	}
	return Result{Entries: out}, nil
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
