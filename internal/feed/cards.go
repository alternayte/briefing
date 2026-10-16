package feed

import (
	"context"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/gx"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/ingest"
)

// Card is one story in the feed.
type Card struct {
	StoryID    int64
	Headline   string
	Summary    string
	LinkCard   bool
	LinkURL    gx.URL
	Concepts   []string
	Sources    []SourceLink
	Publishers int
	Confidence string
	Reason     string
}

// SourceLink is one publisher of a story.
type SourceLink struct {
	Domain  string
	URL     gx.URL
	Favicon gx.URL
}

const cardSelect = `
	SELECT s.id, f.reason, s.publisher_count,
		COALESCE(m.headline, ''), COALESCE(m.summary, ''), COALESCE(m.confidence::text, ''),
		COALESCE((SELECT array_agg(c.name ORDER BY c.name) FROM story_concepts sc JOIN concepts c ON c.id = sc.concept_id WHERE sc.story_id = s.id), '{}'),
		(SELECT array_agg(i.canonical_url ORDER BY i.text_len DESC, i.id) FROM story_items si JOIN items i ON i.id = si.item_id WHERE si.story_id = s.id),
		COALESCE(top.title, ''), COALESCE(top.snippet, '')
	FROM feed_entries f
	JOIN stories s ON s.id = f.story_id
	LEFT JOIN LATERAL (SELECT headline, summary, confidence FROM summaries WHERE story_id = s.id ORDER BY revision DESC LIMIT 1) m ON true
	LEFT JOIN LATERAL (
		SELECT i.title, i.snippet FROM story_items si JOIN items i ON i.id = si.item_id
		WHERE si.story_id = s.id ORDER BY (i.triage->>'significance')::float8 DESC NULLS LAST, i.id LIMIT 1
	) top ON true`

// todayCards returns the stories delivered to the reader today, best first.
func todayCards(ctx context.Context, tx *drel.Tx, userID uuid.UUID, today time.Time, msg i18n.Messages) ([]Card, error) {
	return queryCards(ctx, tx, msg, cardSelect+`
		WHERE f.user_id = $1 AND f.delivered_on = $2::date
		ORDER BY f.score DESC, f.id`, userID, today.Format(time.DateOnly))
}

// moreCards returns the best candidates that did not fit today's budget.
func moreCards(ctx context.Context, tx *drel.Tx, userID uuid.UUID, since time.Time, limit int, msg i18n.Messages) ([]Card, error) {
	return queryCards(ctx, tx, msg, cardSelect+`
		WHERE f.user_id = $1 AND f.state = 'candidate' AND s.last_item_at >= $2 AND f.score > 0
		ORDER BY f.score DESC, f.id LIMIT $3`, userID, since, limit)
}

func queryCards(ctx context.Context, tx *drel.Tx, msg i18n.Messages, sql string, args ...any) ([]Card, error) {
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Card
	for rows.Next() {
		var c Card
		var confidence, title, snippet string
		var urls []string
		if err := rows.Scan(&c.StoryID, &c.Reason, &c.Publishers, &c.Headline, &c.Summary, &confidence, &c.Concepts, &urls, &title, &snippet); err != nil {
			return nil, err
		}
		c.Sources = sourceLinks(urls)
		if c.Headline == "" {
			// Below the evidence floor: a link card with the best item.
			c.LinkCard, c.Headline, c.Summary = true, title, snippet
			if len(c.Sources) > 0 {
				c.LinkURL = c.Sources[0].URL
			}
		}
		c.Confidence = map[string]string{"high": msg.ConfidenceHigh, "medium": msg.ConfidenceMedium, "low": msg.ConfidenceLow}[confidence]
		out = append(out, c)
	}
	return out, rows.Err()
}

// sourceLinks keeps the first URL of each publisher, at most four.
func sourceLinks(urls []string) []SourceLink {
	var out []SourceLink
	seen := map[string]bool{}
	for _, u := range urls {
		d := ingest.RegistrableDomain(u)
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, SourceLink{Domain: d, URL: gx.URL(u), Favicon: gx.URL("https://" + d + "/favicon.ico")})
		if len(out) == 4 {
			break
		}
	}
	return out
}
