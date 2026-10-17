package feed

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/gx"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/ingest"
	"github.com/alternayte/briefing/internal/learning"
	"github.com/alternayte/briefing/internal/translate"
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
	// Seen marks a story the reader opened or rated; Updated a story with a
	// new revision since.
	Seen    bool
	Updated bool
	// Flagged marks a story the reader flagged as misleading: the card hides
	// its summary for them.
	Flagged bool
	// Level is the confidence level: high, medium, low, or empty.
	Level string
	// Translating marks an English entry whose translation is not ready.
	Translating bool
	// Feedback holds the four feedback forms.
	Feedback []FeedbackButtonProps
	// Meta is the freshness, coverage and confidence line, in the reader's
	// locale.
	Meta   []string
	Reason string
	// Place is where the card shows: placeToday, placeDone or placeMore.
	// Done offers Done on Today; Restore brings back a card from "Done
	// today" that the reader marked done, not one they marked knew or
	// irrelevant.
	Place   string
	Done    DoneButtonProps
	Restore *RestoreButtonProps
}

const (
	placeToday = "today"
	placeDone  = "done"
	placeMore  = "more"
)

// SourceLink is one publisher of a story.
type SourceLink struct {
	Domain  string
	URL     gx.URL
	Favicon gx.URL
}

// view is how one reader sees cards: their language, their time zone and
// the time now.
type view struct {
	locale i18n.Locale
	loc    *time.Location
	now    time.Time
}

// day is the reader's date now, as YYYY-MM-DD.
func (v view) day() string { return accounts.LocalDay(v.now, v.loc) }

// cardSelect loads stories as cards for the reader $1, whether or not the
// stories are in their feed.
const cardSelect = `
	SELECT s.id, COALESCE(f.reason, ''), s.publisher_count, COALESCE(f.state = 'seen', false),
		COALESCE(f.shown_revision > 0 AND f.shown_revision < s.revision, false),
		COALESCE(m.headline, ''), COALESCE(m.summary, ''), COALESCE(m.confidence::text, ''),
		COALESCE((SELECT array_agg(c.name ORDER BY c.name) FROM story_concepts sc JOIN concepts c ON c.id = sc.concept_id WHERE sc.story_id = s.id), '{}'),
		(SELECT array_agg(i.canonical_url ORDER BY i.text_len DESC, i.id) FROM story_items si JOIN items i ON i.id = si.item_id WHERE si.story_id = s.id),
		COALESCE(top.title, ''), COALESCE(top.snippet, ''),
		COALESCE((SELECT array_agg(DISTINCT e.kind::text) FROM learning_events e WHERE e.user_id = $1 AND e.story_id = s.id
			AND e.kind IN ('useful', 'knew', 'irrelevant', 'misleading')), '{}'),
		COALESCE(f.state::text, ''), s.last_item_at
	FROM stories s
	LEFT JOIN feed_entries f ON f.story_id = s.id AND f.user_id = $1
	LEFT JOIN LATERAL (SELECT headline, summary, confidence FROM summaries WHERE story_id = s.id ORDER BY revision DESC LIMIT 1) m ON true
	LEFT JOIN LATERAL (
		SELECT i.title, i.snippet FROM story_items si JOIN items i ON i.id = si.item_id
		WHERE si.story_id = s.id ORDER BY (i.triage->>'significance')::float8 DESC NULLS LAST, i.id LIMIT 1
	) top ON true`

// returned holds for a done entry that is back on Today: its story has a
// revision the reader has not seen, and they did not mark it knew or
// irrelevant.
const returned = `(f.shown_revision < s.revision AND NOT EXISTS (SELECT 1 FROM learning_events e
	WHERE e.user_id = f.user_id AND e.story_id = f.story_id AND e.kind IN ('knew', 'irrelevant')))`

// todayCards returns the stories delivered to the reader on their current
// day that are not done, best first.
func todayCards(ctx context.Context, tx *drel.Tx, userID uuid.UUID, v view) ([]Card, error) {
	return placed(placeToday)(queryCards(ctx, tx, v, cardSelect+`
		WHERE f.delivered_on = $2::date AND (f.state <> 'done' OR `+returned+`)
		ORDER BY f.score DESC, f.id`, userID, v.day()))
}

// doneCards returns the done stories of the reader's current day, the last
// one done first.
func doneCards(ctx context.Context, tx *drel.Tx, userID uuid.UUID, v view) ([]Card, error) {
	return placed(placeDone)(queryCards(ctx, tx, v, cardSelect+`
		WHERE f.delivered_on = $2::date AND f.state = 'done' AND NOT `+returned+`
		ORDER BY f.updated_at DESC, f.id`, userID, v.day()))
}

// moreCards returns the best candidates that did not fit today's budget.
func moreCards(ctx context.Context, tx *drel.Tx, userID uuid.UUID, since time.Time, limit int, v view) ([]Card, error) {
	return placed(placeMore)(queryCards(ctx, tx, v, cardSelect+`
		WHERE f.state = 'candidate' AND s.last_item_at >= $2 AND f.score > 0
		ORDER BY f.score DESC, f.id LIMIT $3`, userID, since, limit))
}

// placed sets the place of every card, and the Done form on Today.
func placed(place string) func([]Card, error) ([]Card, error) {
	return func(cards []Card, err error) ([]Card, error) {
		for i := range cards {
			cards[i].Place = place
			if place != placeDone {
				cards[i].Restore = nil
			}
		}
		return cards, err
	}
}

func queryCards(ctx context.Context, tx *drel.Tx, v view, sql string, args ...any) ([]Card, error) {
	l := v.locale
	msg := i18n.For(l)
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Card
	for rows.Next() {
		var c Card
		var confidence, title, snippet string
		var urls, given []string
		var state string
		var lastItemAt time.Time
		if err := rows.Scan(&c.StoryID, &c.Reason, &c.Publishers, &c.Seen, &c.Updated, &c.Headline, &c.Summary, &confidence, &c.Concepts, &urls, &title, &snippet, &given,
			&state, &lastItemAt); err != nil {
			return nil, err
		}
		c.Flagged = slices.Contains(given, string(learning.EventMisleading))
		c.Meta = append(c.Meta, i18n.Ago(l, lastItemAt, v.now, v.loc))
		c.Sources = sourceLinks(urls)
		c.Level = confidence
		if c.Headline == "" {
			// Below the evidence floor: a link card with the best item.
			c.LinkCard, c.Headline, c.Summary = true, title, snippet
			if len(c.Sources) > 0 {
				c.LinkURL = c.Sources[0].URL
			}
		}
		if c.Publishers > 1 {
			c.Meta = append(c.Meta, fmt.Sprintf(msg.FeedPublishers, c.Publishers))
		}
		if word := map[string]string{"high": msg.ConfidenceHigh, "medium": msg.ConfidenceMedium, "low": msg.ConfidenceLow}[confidence]; word != "" {
			c.Meta = append(c.Meta, fmt.Sprintf(msg.FeedConfidence, word))
		}
		if c.Reason != "" {
			c.Reason = fmt.Sprintf(msg.FeedBecause, c.Reason)
		}
		c.Feedback = feedbackButtons(c.StoryID, "", given, feedbackLabels(msg))
		c.Done = doneButton(c.StoryID, false, msg.FeedDone)
		if state == string(EntryDone) && !slices.Contains(given, string(learning.EventKnew)) && !slices.Contains(given, string(learning.EventIrrelevant)) {
			r := restoreButton(c.StoryID, msg.FeedRestore)
			c.Restore = &r
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if l == i18n.Locales[0] {
		return out, nil
	}
	for i := range out {
		if out[i].LinkCard {
			continue
		}
		f, ok, err := translate.Story(ctx, tx, out[i].StoryID, l)
		if err != nil {
			return nil, err
		}
		if !ok {
			out[i].Translating = true
			continue
		}
		out[i].Headline, out[i].Summary, out[i].Concepts = f.Headline, f.Summary, nil
		for _, c := range f.Concepts {
			out[i].Concepts = append(out[i].Concepts, c.Name)
		}
	}
	return out, nil
}

// cardsByIDs loads the given stories as cards, in the order of ids.
func cardsByIDs(ctx context.Context, tx *drel.Tx, userID uuid.UUID, ids []int64, v view) ([]Card, error) {
	cards, err := queryCards(ctx, tx, v, cardSelect+`
		WHERE s.id = ANY($2)`, userID, ids)
	if err != nil {
		return nil, err
	}
	at := map[int64]int{}
	for i, id := range ids {
		at[id] = i
	}
	slices.SortFunc(cards, func(a, b Card) int { return at[a.StoryID] - at[b.StoryID] })
	return cards, nil
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

func feedbackLabels(msg i18n.Messages) [4]string {
	return [4]string{msg.FeedbackUseful, msg.FeedbackKnew, msg.FeedbackIrrelevant, msg.FeedbackMisleading}
}
