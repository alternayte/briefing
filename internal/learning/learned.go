package learning

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
	"github.com/alternayte/briefing/internal/learning/route"
	"github.com/alternayte/briefing/internal/translate"
)

// LearnedConcept is one concept the reader met.
type LearnedConcept struct {
	ID         int64       `json:"id"`
	Name       string      `json:"name"`
	Definition string      `json:"definition"`
	State      string      `json:"state"`
	Stories    []StoryLink `json:"stories"`
	lastAt     time.Time
	state      ConceptState
}

// StoryLink is a story a concept came from.
type StoryLink struct {
	ID       int64  `json:"id"`
	Headline string `json:"headline"`
	URL      gx.URL `json:"-"`
}

var stateNames = map[ConceptState]string{StateMastered: "mastered", StateFamiliar: "familiar", StateMet: "met"}

// learned returns the reader's concepts, the strongest state first and the
// newest first within a state.
func learned(ctx context.Context, tx *drel.Tx, userID uuid.UUID, l i18n.Locale) ([]LearnedConcept, error) {
	states, err := ConceptStates(ctx, tx, userID)
	if err != nil || len(states) == 0 {
		return nil, err
	}
	ids := make([]int64, 0, len(states))
	for id, s := range states {
		if s != StateNone {
			ids = append(ids, id)
		}
	}
	rows, err := tx.Query(ctx, `
		SELECT c.id, c.name, c.definition, max(e.created_at),
			(array_agg(DISTINCT s.id))[1:3],
			(SELECT array_agg(m.headline ORDER BY m.story_id) FROM (
				SELECT DISTINCT ON (sm.story_id) sm.story_id, sm.headline FROM summaries sm
				JOIN story_concepts x ON x.story_id = sm.story_id AND x.concept_id = c.id
				JOIN learning_events y ON y.story_id = sm.story_id AND y.user_id = $1
				ORDER BY sm.story_id, sm.revision DESC LIMIT 3) m)
		FROM concepts c
		JOIN story_concepts sc ON sc.concept_id = c.id
		JOIN learning_events e ON e.story_id = sc.story_id AND e.user_id = $1
		JOIN stories s ON s.id = sc.story_id
		WHERE c.id = ANY($2)
		GROUP BY c.id`, userID, ids)
	if err != nil {
		return nil, err
	}
	var out []LearnedConcept
	for rows.Next() {
		var c LearnedConcept
		var storyIDs []int64
		var headlines []string
		if err := rows.Scan(&c.ID, &c.Name, &c.Definition, &c.lastAt, &storyIDs, &headlines); err != nil {
			rows.Close()
			return nil, err
		}
		slices.Sort(storyIDs)
		for i, id := range storyIDs {
			link := StoryLink{ID: id, URL: gx.URL(fmt.Sprintf("/s/%d", id))}
			if i < len(headlines) {
				link.Headline = headlines[i]
			}
			if link.Headline != "" {
				c.Stories = append(c.Stories, link)
			}
		}
		c.state = states[c.ID]
		c.State = stateNames[c.state]
		out = append(out, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := translateConcepts(ctx, tx, out, l); err != nil {
		return nil, err
	}
	if l != i18n.Locales[0] {
		for i := range out {
			for j := range out[i].Stories {
				f, ok, err := translate.Story(ctx, tx, out[i].Stories[j].ID, l)
				if err != nil {
					return nil, err
				}
				if ok {
					out[i].Stories[j].Headline = f.Headline
				}
			}
		}
	}
	slices.SortFunc(out, func(a, b LearnedConcept) int {
		if a.state != b.state {
			return int(b.state) - int(a.state)
		}
		return b.lastAt.Compare(a.lastAt)
	})
	return out, nil
}

// translateConcepts uses any translation of a concept from a story it was in.
func translateConcepts(ctx context.Context, tx *drel.Tx, cs []LearnedConcept, l i18n.Locale) error {
	if l == i18n.Locales[0] || len(cs) == 0 {
		return nil
	}
	ids := make([]int64, len(cs))
	for i, c := range cs {
		ids[i] = c.ID
	}
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT ON ((c->>'id')::bigint) (c->>'id')::bigint, c->>'name', c->>'definition'
		FROM translations t, jsonb_array_elements(t.fields->'concepts') c
		WHERE t.kind = 'story' AND t.locale = $1 AND (c->>'id')::bigint = ANY($2)`, string(l), ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	byID := map[int64]translate.ConceptText{}
	for rows.Next() {
		var c translate.ConceptText
		if err := rows.Scan(&c.ID, &c.Name, &c.Definition); err != nil {
			return err
		}
		byID[c.ID] = c
	}
	for i := range cs {
		if c, ok := byID[cs[i].ID]; ok {
			cs[i].Name, cs[i].Definition = c.Name, c.Definition
		}
	}
	return rows.Err()
}

// WeekCounts are the reader's numbers for the last seven days.
type WeekCounts struct {
	StoriesRead      int
	ConceptsMet      int
	ConceptsMastered int
}

func weekCounts(ctx context.Context, tx *drel.Tx, userID uuid.UUID, now time.Time, concepts []LearnedConcept) (WeekCounts, error) {
	var w WeekCounts
	since := now.Add(-7 * 24 * time.Hour)
	err := tx.QueryRow(ctx, `
		SELECT
			(SELECT count(DISTINCT story_id) FROM learning_events WHERE user_id = $1 AND kind = 'opened' AND created_at >= $2),
			(SELECT count(*) FROM (
				SELECT sc.concept_id FROM learning_events e JOIN story_concepts sc ON sc.story_id = e.story_id
				WHERE e.user_id = $1 GROUP BY sc.concept_id HAVING min(e.created_at) >= $2) first_met)`,
		userID, since).Scan(&w.StoriesRead, &w.ConceptsMet)
	for _, c := range concepts {
		if c.state == StateMastered && !c.lastAt.Before(since) {
			w.ConceptsMastered++
		}
	}
	return w, err
}

// ConceptGroup is the concepts of one state.
type ConceptGroup struct {
	Label    string
	Concepts []LearnedConcept
}

// LearnedPageHandler shows the concepts by state with the week's numbers.
var LearnedPageHandler = gx.Page(func(c *gx.Ctx, _ route.LearnedPage) (LearnedViewProps, error) {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	locale := i18n.FromContext(ctx)
	msg := i18n.For(locale)
	p := LearnedViewProps{Msg: msg}
	err := w.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		concepts, err := learned(ctx, tx, profile.ID(), locale)
		if err != nil {
			return err
		}
		week, err := weekCounts(ctx, tx, profile.ID(), w.now(), concepts)
		if err != nil {
			return err
		}
		p.Week = []string{
			i18n.Count(locale, week.StoriesRead, msg.LearnedStoriesReadOne, msg.LearnedStoriesRead),
			i18n.Count(locale, week.ConceptsMet, msg.LearnedConceptsMetOne, msg.LearnedConceptsMet),
			i18n.Count(locale, week.ConceptsMastered, msg.LearnedConceptsMasteredOne, msg.LearnedConceptsMastered),
		}
		labels := map[ConceptState]string{StateMastered: msg.ConceptMastered, StateFamiliar: msg.ConceptFamiliar, StateMet: msg.ConceptMet}
		for _, st := range []ConceptState{StateMastered, StateFamiliar, StateMet} {
			g := ConceptGroup{Label: labels[st]}
			for _, c := range concepts {
				if c.state == st {
					g.Concepts = append(g.Concepts, c)
				}
			}
			if len(g.Concepts) > 0 {
				p.Groups = append(p.Groups, g)
			}
		}
		return nil
	})
	return p, err
}, LearnedView)

// Lists the concepts the reader met in their reading, each with its state
// (met, familiar or mastered), its definition and up to three stories it came
// from. Use it to see what the reader already knows before explaining
// something, or when they ask what they have learned.
var LearnedAction = gx.Action(func(c *gx.Ctx, _ route.Learned) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	return w.engine.WithTx(ctx, func(ctx context.Context) error {
		concepts, err := learned(ctx, drel.MustFromContext(ctx), profile.ID(), i18n.FromContext(ctx))
		if concepts == nil {
			concepts = []LearnedConcept{}
		}
		gx.ToolResult(c, concepts)
		return err
	})
}).Tool()

// Routes lists every page, action and form of the slice.
var Routes = gx.Collect(ReviewPage, ReviewDueAction, ReviewAnswer, LearnedPageHandler, LearnedAction)
