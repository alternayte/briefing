package sources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/alternayte/drel"
	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"

	"github.com/alternayte/briefing/internal/platform"
)

var (
	// ErrFeedURL reports a URL that is not an absolute http(s) address.
	ErrFeedURL = errors.New("not an absolute http or https URL")
	// ErrFeedUnreachable reports a feed that could not be fetched, including
	// one on a private address.
	ErrFeedUnreachable = errors.New("the feed could not be fetched")
	// ErrNotAFeed reports a page that is not RSS or Atom.
	ErrNotAFeed = errors.New("the page is not an RSS or Atom feed")
)

// AddFeed checks that rawURL is a reachable feed on the public internet,
// adds it to the catalog as a tier-3 source and subscribes the reader.
func AddFeed(ctx context.Context, engine *drel.Engine, client *http.Client, t platform.Tuning, userID uuid.UUID, rawURL string) (Listed, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Listed{}, ErrFeedURL
	}
	u.Fragment = ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Listed{}, ErrFeedURL
	}
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml;q=0.9, */*;q=0.5")
	resp, err := client.Do(req)
	if err != nil {
		return Listed{}, fmt.Errorf("%w: %w", ErrFeedUnreachable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Listed{}, fmt.Errorf("%w: status %d", ErrFeedUnreachable, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, t.Poll.MaxBytes))
	if err != nil {
		return Listed{}, fmt.Errorf("%w: %w", ErrFeedUnreachable, err)
	}
	feed, err := gofeed.NewParser().ParseString(string(body))
	if err != nil {
		return Listed{}, ErrNotAFeed
	}
	title := strings.TrimSpace(feed.Title)
	if title == "" {
		title = u.Hostname()
	}
	l := Listed{Kind: string(KindRSS), Title: title, URL: u.String(), Tier: 3, Subscribed: true}
	err = engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		if err := tx.QueryRow(ctx, `
			INSERT INTO sources (kind, url, title, tier, language, enabled, poll_interval_seconds)
			VALUES ('rss', $1, $2, 3, 'en', true, $3)
			ON CONFLICT (kind, url) DO UPDATE SET updated_at = now()
			RETURNING id, title, tier`, l.URL, title, int(t.Poll.RSS.Seconds())).Scan(&l.ID, &l.Title, &l.Tier); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO subscriptions (user_id, source_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, l.ID)
		return err
	})
	return l, err
}
