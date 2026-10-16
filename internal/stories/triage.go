package stories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alternayte/drel"

	"github.com/alternayte/briefing/internal/decide"
	"github.com/alternayte/briefing/internal/ingest"
	"github.com/alternayte/briefing/internal/platform"
)

const triageConcurrency = 4

// categoryGuide describes each taxonomy slug for the decision model.
var categoryGuide = map[string]string{
	"ai_ml":                 "AI and machine learning",
	"software_engineering":  "Software engineering: tools, practices, infrastructure software",
	"programming_languages": "Programming languages, compilers and runtimes",
	"security":              "Security: vulnerabilities, breaches, privacy, cryptography",
	"infrastructure_cloud":  "Infrastructure and cloud: networks, data centres, operations",
	"hardware_chips":        "Hardware and chips",
	"space":                 "Space: missions, astronomy, space industry",
	"physics":               "Physics",
	"biology_medicine":      "Biology and medicine",
	"climate_energy":        "Climate and energy",
	"mathematics":           "Mathematics",
	"science_policy":        "Science and technology policy, funding and regulation",
	platform.CategoryOther:  "Anything else, including general business, politics, culture and consumer deals",
}

func triageQuestions(taxonomy []string) decide.Questions {
	cats := map[string]any{}
	for _, slug := range taxonomy {
		cats[slug] = categoryGuide[slug]
	}
	return decide.Questions{
		"worth": decide.Noul(
			"Does this report a concrete development, finding, release or event that a curious professional would want to know? Promotions, listicles, opinion without news, job posts and rehashes are false.",
			"It reports a concrete development, finding, release or event.",
			"It is a promotion, listicle, opinion without news, job post or rehash."),
		"sensational": decide.Noul("Is the headline sensational or clickbait?",
			"The headline exaggerates or withholds information to get clicks.",
			"The headline states what happened plainly."),
		"category":     {Type: "choice", Instructions: "Which topic does this item belong to?", Criteria: cats},
		"significance": decide.Score("How significant is this development?", "trivial", "minor", "notable", "major", "landmark"),
		"learning":     decide.Score("How much would a curious reader learn from it?", "nothing new", "small detail", "useful insight", "substantial new knowledge"),
	}
}

// Triager asks the decision model about items that passed the rules.
type Triager struct {
	engine    *drel.Engine
	decider   *decide.Decider
	questions decide.Questions
	worthMin  float64
	now       func() time.Time
}

func NewTriager(engine *drel.Engine, d *decide.Decider, t platform.Tuning, now func() time.Time) *Triager {
	return &Triager{engine: engine, decider: d, questions: triageQuestions(t.Taxonomy), worthMin: t.Triage.WorthMin, now: now}
}

// Triage decides each passed item among ids and returns the ids it kept. It
// saves every decision it got and hands the kept items to next in one
// transaction, so a failed call leaves the others done and a retry asks only
// about the rest.
func (t *Triager) Triage(ctx context.Context, ids []int64, next platform.Next) ([]int64, error) {
	items, err := t.passed(ctx, ids)
	if err != nil {
		return nil, err
	}
	limit := triageConcurrency
	if _, inTx := drel.FromContext(ctx); inTx {
		// One transaction is one connection; its users must take turns.
		limit = 1
	}
	var (
		mu      sync.Mutex
		decided = map[int64]*ingest.Triage{}
		kept    []int64
		errs    []error
		wg      sync.WaitGroup
		sem     = make(chan struct{}, limit)
	)
	for _, it := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			tr, err := t.one(ctx, it)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("item %d: %w", it.item.ID(), err))
				return
			}
			decided[it.item.ID()] = tr
			if t.keeps(tr) {
				kept = append(kept, it.item.ID())
			}
		}()
	}
	wg.Wait()
	if len(decided) == 0 {
		return nil, errors.Join(errs...)
	}
	err = t.engine.WithTx(ctx, func(ctx context.Context) error {
		ids := make([]int64, 0, len(decided))
		for id := range decided {
			ids = append(ids, id)
		}
		items, err := drel.NewTxRepository(drel.MustFromContext(ctx), ingest.ItemMeta).Where(ingest.Items.ID.In(ids...)).All(ctx)
		if err != nil {
			return err
		}
		for _, it := range items {
			it.Triage, it.Status = decided[it.ID()], ingest.ItemRejected
			if t.keeps(it.Triage) {
				it.Status = ingest.ItemKept
			}
		}
		if len(kept) == 0 || next == nil {
			return nil
		}
		return next(ctx, kept)
	})
	if err != nil {
		return nil, fmt.Errorf("store triage: %w", err)
	}
	return kept, errors.Join(errs...)
}

func (t *Triager) keeps(tr *ingest.Triage) bool {
	return tr.Worth >= t.worthMin && tr.Category != platform.CategoryOther
}

type triageInput struct {
	item    *ingest.Item
	sources []string
}

func (t *Triager) passed(ctx context.Context, ids []int64) ([]triageInput, error) {
	var out []triageInput
	err := t.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		items, err := drel.NewTxRepository(tx, ingest.ItemMeta).
			Where(drel.And(ingest.Items.ID.In(ids...), ingest.Items.Status.Eq(ingest.ItemPassed))).All(ctx)
		if err != nil {
			return err
		}
		for _, it := range items {
			names, err := sourceTitles(ctx, tx, it.ID())
			if err != nil {
				return err
			}
			out = append(out, triageInput{item: it, sources: names})
		}
		return nil
	})
	return out, err
}

func sourceTitles(ctx context.Context, tx *drel.Tx, itemID int64) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT DISTINCT s.title FROM sightings g JOIN sources s ON s.id = g.source_id WHERE g.item_id = $1 ORDER BY 1`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

func (t *Triager) one(ctx context.Context, in triageInput) (*ingest.Triage, error) {
	state := fmt.Sprintf("Title: %s\nSource: %s\nSnippet: %s", in.item.Title, strings.Join(in.sources, ", "), in.item.Snippet)
	ans, err := t.decider.Ask(ctx, "triage", state, t.questions)
	if err != nil {
		return nil, err
	}
	return &ingest.Triage{
		Worth:        ans["worth"].Noul,
		Sensational:  ans["sensational"].Noul,
		Category:     ans["category"].Choice,
		Significance: ans["significance"].Score,
		Learning:     ans["learning"].Score,
		Model:        t.decider.Model(),
		DecidedAt:    t.now().UTC(),
	}, nil
}
