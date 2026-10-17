package feed

import (
	"context"
	"fmt"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/gx"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/feed/route"
)

const (
	searchLimit      = 10
	searchDefaultAge = 30 * 24 * time.Hour
)

// StoryJSON is a story as an assistant gets it. Text is in the reader's
// language when a translation exists.
type StoryJSON struct {
	ID         int64        `json:"id"`
	URL        string       `json:"url"`
	Headline   string       `json:"headline"`
	Summary    string       `json:"summary"`
	LinkOnly   bool         `json:"link_only,omitempty"`
	Concepts   []string     `json:"concepts,omitempty"`
	Sources    []SourceJSON `json:"sources"`
	Publishers int          `json:"publishers"`
	Confidence string       `json:"confidence,omitempty"`
	Reason     string       `json:"reason,omitempty"`
	Seen       bool         `json:"seen"`
	Updated    bool         `json:"updated,omitempty"`
	// Done marks a story the reader took off Today; Flagged one they marked
	// misleading.
	Done    bool `json:"done,omitempty"`
	Flagged bool `json:"flagged,omitempty"`
}

type SourceJSON struct {
	Publisher string `json:"publisher"`
	URL       string `json:"url"`
}

func (w *Web) storyJSON(c Card) StoryJSON {
	s := StoryJSON{
		ID: c.StoryID, URL: fmt.Sprintf("%s/s/%d", w.baseURL, c.StoryID), Headline: c.Headline, Summary: c.Summary,
		LinkOnly: c.LinkCard, Concepts: c.Concepts, Publishers: c.Publishers, Confidence: c.Level,
		Reason: c.Reason, Seen: c.Seen, Updated: c.Updated, Done: c.Place == placeDone, Flagged: c.Flagged, Sources: []SourceJSON{},
	}
	for _, src := range c.Sources {
		s.Sources = append(s.Sources, SourceJSON{Publisher: src.Domain, URL: string(src.URL)})
	}
	return s
}

// BriefJSON is today's brief.
type BriefJSON struct {
	Date       string      `json:"date"`
	Language   string      `json:"language"`
	Today      []StoryJSON `json:"today"`
	More       []StoryJSON `json:"more"`
	ReviewsDue int         `json:"reviews_due"`
}

// Returns the reader's brief for today, their local day: the stories
// delivered today, best first, then the ones they took off Today with done
// true, then more stories that did not fit the daily budget. Each story has
// its id, a link, the headline and summary in the reader's language, its
// concepts, its sources and why it was chosen. reviews_due counts the review
// questions waiting. Use it for a morning briefing or "what's new".
var TodayAction = gx.Action(func(c *gx.Ctx, _ route.Today) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	v := w.viewOf(ctx, profile)
	brief := BriefJSON{Date: v.day(), Language: string(v.locale), Today: []StoryJSON{}, More: []StoryJSON{}}
	err := w.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		today, err := todayCards(ctx, tx, profile.ID(), v)
		if err != nil {
			return err
		}
		done, err := doneCards(ctx, tx, profile.ID(), v)
		if err != nil {
			return err
		}
		more, err := moreCards(ctx, tx, profile.ID(), v.now.Add(-w.tuning.Cluster.OpenWindow), moreLimit, v)
		if err != nil {
			return err
		}
		for _, card := range append(today, done...) {
			brief.Today = append(brief.Today, w.storyJSON(card))
		}
		for _, card := range more {
			brief.More = append(brief.More, w.storyJSON(card))
		}
		brief.ReviewsDue, err = dueCount(ctx, tx, profile.ID(), v, w)
		return err
	})
	gx.ToolResult(c, brief)
	return err
}).Tool()

// StoryDetailJSON is one full story.
type StoryDetailJSON struct {
	StoryJSON
	WhyItMatters string        `json:"why_it_matters,omitempty"`
	WhatIsNew    string        `json:"what_is_new,omitempty"`
	Claims       []ClaimJSON   `json:"claims,omitempty"`
	Definitions  []ConceptJSON `json:"concept_definitions,omitempty"`
	Explanation  string        `json:"explanation,omitempty"`
}

type ClaimJSON struct {
	Text    string   `json:"text"`
	Sources []string `json:"sources"`
}

type ConceptJSON struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
	State      string `json:"reader_state"`
}

// Returns one story in full by its id: headline, summary, why it matters,
// what is new, each claim with the source URLs that state it, its concepts
// with definitions and how well the reader knows each, its sources, and the
// reader's explanation when they asked for one. Counts as the reader opening
// the story. Use it after feed_today or feed_search to go deeper.
var StoryAction = gx.Action(func(c *gx.Ctx, in route.Story) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	v := w.viewOf(ctx, profile)
	locale := v.locale
	var out StoryDetailJSON
	found := false
	err := w.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		cards, err := cardsByIDs(ctx, tx, profile.ID(), []int64{in.ID}, v)
		if err != nil || len(cards) == 0 {
			return err
		}
		var p StoryViewProps
		if found, err = loadStory(ctx, tx, in.ID, profile.ID(), v, &p); err != nil || !found {
			return err
		}
		out = StoryDetailJSON{StoryJSON: w.storyJSON(cards[0]), WhyItMatters: p.Why, WhatIsNew: p.New}
		for _, cl := range p.Claims {
			cj := ClaimJSON{Text: cl.Text, Sources: []string{}}
			for _, r := range cl.Refs {
				cj.Sources = append(cj.Sources, string(r.URL))
			}
			out.Claims = append(out.Claims, cj)
		}
		for _, cv := range p.Concepts {
			out.Definitions = append(out.Definitions, ConceptJSON{Name: cv.Name, Definition: cv.Definition, State: cv.State})
		}
		return markOpened(ctx, tx, profile.ID(), in.ID)
	})
	if err != nil {
		return err
	}
	if !found {
		return gx.NotFound()
	}
	if w.explainer != nil {
		if out.Explanation, err = w.explainer.Cached(ctx, profile.ID(), in.ID, locale); err != nil {
			return err
		}
	}
	gx.ToolResult(c, out)
	return nil
}).Tool()

// SearchJSON is the result of a search.
type SearchJSON struct {
	Query   string      `json:"query"`
	From    string      `json:"from"`
	To      string      `json:"to"`
	Stories []StoryJSON `json:"stories"`
}

// Finds stories by meaning, not only by words, between two dates (from and
// to as YYYY-MM-DD; by default the last 30 days). Returns up to 10 stories,
// best match first, in the same shape as feed_today. Use it when the reader
// asks about a topic, a company or an event.
var SearchAction = gx.Action(func(c *gx.Ctx, in route.Search) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	now := w.now().UTC()
	to, err := parseDay(in.To, now)
	if err != nil {
		return gx.FieldError(&in.To, "date")
	}
	from, err := parseDay(in.From, to.Add(-searchDefaultAge))
	if err != nil {
		return gx.FieldError(&in.From, "date")
	}
	res := SearchJSON{Query: in.Q, From: from.Format(time.DateOnly), To: to.Format(time.DateOnly), Stories: []StoryJSON{}}
	vs, err := w.embedder.Embed(ctx, "search", []string{in.Q})
	if err != nil {
		return err
	}
	q, err := vs[0].Value()
	if err != nil {
		return err
	}
	err = w.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		ids, err := nearestStories(ctx, tx, q, from, to.Add(24*time.Hour))
		if err != nil || len(ids) == 0 {
			return err
		}
		cards, err := cardsByIDs(ctx, tx, profile.ID(), ids, w.viewOf(ctx, profile))
		for _, card := range cards {
			res.Stories = append(res.Stories, w.storyJSON(card))
		}
		return err
	})
	gx.ToolResult(c, res)
	return err
}).Tool()

func nearestStories(ctx context.Context, tx *drel.Tx, q any, from, to time.Time) ([]int64, error) {
	rows, err := tx.Query(ctx, `
		SELECT s.id FROM stories s
		WHERE s.last_item_at >= $2 AND s.last_item_at < $3 AND s.centroid IS NOT NULL
			AND vector_dims(s.centroid) = vector_dims($1::vector)
		ORDER BY s.centroid <=> $1::vector
		LIMIT $4`, q, from, to, searchLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func parseDay(s string, fallback time.Time) (time.Time, error) {
	if s == "" {
		return fallback.Truncate(24 * time.Hour), nil
	}
	return time.Parse(time.DateOnly, s)
}

func dueCount(ctx context.Context, tx *drel.Tx, userID uuid.UUID, v view, w *Web) (int, error) {
	due, err := learningDue(ctx, tx, userID, v.now, v.loc, w)
	return len(due), err
}
