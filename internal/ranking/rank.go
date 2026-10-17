package ranking

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alternayte/drel"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/decide"
	"github.com/alternayte/briefing/internal/learning"
	"github.com/alternayte/briefing/internal/platform"
)

const decideConcurrency = 4

var relevanceQuestions = decide.Questions{
	"relevance": decide.Score("How relevant is this story to the reader's interests?",
		"unrelated", "tangential", "related", "core interest"),
	"excluded": decide.Noul("Is this story mainly about a topic the reader does not want?",
		"Its main subject is one of the topics the reader does not want.",
		"Its main subject is not one of those topics, or the reader listed none."),
}

// Ranker fills one reader's feed.
type Ranker struct {
	engine  *drel.Engine
	decider *decide.Decider
	tuning  platform.Tuning
	now     func() time.Time
}

func NewRanker(engine *drel.Engine, d *decide.Decider, t platform.Tuning, now func() time.Time) *Ranker {
	return &Ranker{engine: engine, decider: d, tuning: t, now: now}
}

// Result says what a run found.
type Result struct {
	// Relevant are the stories that cleared the reader's prefilter in this
	// run; they get fetched and written even when not notable.
	Relevant  []int64
	Delivered int64
}

type candidate struct {
	storyID  int64
	interest string
	headline string
	text     string
}

// Rank decides the new stories, scores every entry of the window and
// delivers the best ones up to today's budget.
func (r *Ranker) Rank(ctx context.Context, userID uuid.UUID) (Result, error) {
	var res Result
	now := r.now().UTC()
	positives, negatives, budget, tz, err := r.reader(ctx, userID)
	if err != nil || len(positives) == 0 {
		return res, err
	}
	cands, err := r.prefilter(ctx, userID, now)
	if err != nil {
		return res, err
	}
	for _, c := range cands {
		res.Relevant = append(res.Relevant, c.storyID)
	}
	if err := r.decideAll(ctx, userID, cands, positives, negatives); err != nil {
		return res, err
	}
	if err := r.rescore(ctx, userID, now); err != nil {
		return res, err
	}
	res.Delivered, err = r.deliver(ctx, userID, budget, accounts.LocalDay(now, accounts.Location(tz)), now)
	return res, err
}

func (r *Ranker) reader(ctx context.Context, userID uuid.UUID) (pos, neg []string, budget int, tz string, err error) {
	err = r.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		if err := tx.QueryRow(ctx, `SELECT daily_budget, timezone FROM profiles WHERE id = $1`, userID).Scan(&budget, &tz); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT text, negative FROM interests WHERE user_id = $1 ORDER BY weight DESC, id`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var text string
			var negative bool
			if err := rows.Scan(&text, &negative); err != nil {
				return err
			}
			if negative {
				neg = append(neg, text)
			} else {
				pos = append(pos, text)
			}
		}
		return rows.Err()
	})
	return pos, neg, budget, tz, err
}

// prefilter returns the recent stories, seen by a source the reader
// subscribes to and with no entry for them yet, whose centroid comes close
// enough to one of their positive interests.
func (r *Ranker) prefilter(ctx context.Context, userID uuid.UUID, now time.Time) ([]candidate, error) {
	var out []candidate
	err := r.engine.WithTx(ctx, func(ctx context.Context) error {
		rows, err := drel.MustFromContext(ctx).Query(ctx, `
			WITH matches AS (
				SELECT s.id, i.text, (1 - (s.centroid <=> i.embedding)) * i.weight AS sim
				FROM stories s
				JOIN interests i ON i.user_id = $1 AND NOT i.negative AND i.embedding IS NOT NULL
					AND vector_dims(i.embedding) = vector_dims(s.centroid)
				WHERE s.last_item_at >= $2
					AND NOT EXISTS (SELECT 1 FROM feed_entries f WHERE f.user_id = $1 AND f.story_id = s.id)
					AND EXISTS (
						SELECT 1 FROM story_items si
						JOIN sightings g ON g.item_id = si.item_id
						JOIN subscriptions sub ON sub.source_id = g.source_id AND sub.user_id = $1
						WHERE si.story_id = s.id)
			), best AS (
				SELECT DISTINCT ON (id) id, text, sim FROM matches ORDER BY id, sim DESC
			)
			SELECT b.id, b.text,
				COALESCE(m.headline, top.title), COALESCE(m.summary, top.snippet)
			FROM best b
			LEFT JOIN LATERAL (SELECT headline, summary FROM summaries WHERE story_id = b.id ORDER BY revision DESC LIMIT 1) m ON true
			LEFT JOIN LATERAL (
				SELECT i.title, i.snippet FROM story_items si JOIN items i ON i.id = si.item_id
				WHERE si.story_id = b.id ORDER BY (i.triage->>'significance')::float8 DESC NULLS LAST, i.id LIMIT 1
			) top ON true
			WHERE b.sim >= $3
			ORDER BY b.sim DESC`, userID, now.Add(-r.tuning.Cluster.OpenWindow), r.tuning.Rank.RelevanceMin)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c candidate
			if err := rows.Scan(&c.storyID, &c.interest, &c.headline, &c.text); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Ranker) decideAll(ctx context.Context, userID uuid.UUID, cands []candidate, pos, neg []string) error {
	limit := decideConcurrency
	if _, inTx := drel.FromContext(ctx); inTx {
		limit = 1
	}
	interests := "Interests: " + strings.Join(pos, "; ")
	if len(neg) > 0 {
		interests += "\nDoes not want: " + strings.Join(neg, "; ")
	}
	var (
		mu   sync.Mutex
		errs []error
		wg   sync.WaitGroup
		sem  = make(chan struct{}, limit)
	)
	for _, c := range cands {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			state := fmt.Sprintf("Story: %s\n%s\n\n%s", c.headline, c.text, interests)
			ans, err := r.decider.Ask(ctx, "rank", state, relevanceQuestions)
			if err == nil {
				err = r.engine.WithTx(ctx, func(ctx context.Context) error {
					_, err := drel.MustFromContext(ctx).Exec(ctx, `
						INSERT INTO feed_entries (user_id, story_id, relevance, excluded, score, reason, state)
						VALUES ($1, $2, $3, $4, 0, $5, 'candidate') ON CONFLICT (user_id, story_id) DO NOTHING`,
						userID, c.storyID, ans["relevance"].Score, ans["excluded"].Noul, c.interest)
					return err
				})
			}
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("story %d: %w", c.storyID, err))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}

// rescore recomputes the score of every entry whose story is still in the
// window, so freshness, coverage and what the reader learned stay current.
func (r *Ranker) rescore(ctx context.Context, userID uuid.UUID, now time.Time) error {
	return r.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		states, err := learning.ConceptStates(ctx, tx, userID)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT f.id, f.relevance, f.excluded, s.significance, s.learning, s.sensational, s.publisher_count, s.last_item_at,
				COALESCE((
					SELECT max(1 - (s.centroid <=> k.centroid))
					FROM learning_events e JOIN stories k ON k.id = e.story_id
					WHERE e.user_id = $1 AND e.kind = 'knew' AND e.created_at >= $3 AND k.id <> s.id
						AND vector_dims(k.centroid) = vector_dims(s.centroid)
				), 0),
				COALESCE((SELECT array_agg(concept_id) FROM story_concepts WHERE story_id = s.id), '{}')
			FROM feed_entries f JOIN stories s ON s.id = f.story_id
			WHERE f.user_id = $1 AND s.last_item_at >= $2`,
			userID, now.Add(-r.tuning.Cluster.OpenWindow), now.Add(-r.tuning.Rank.KnewWindow))
		if err != nil {
			return err
		}
		type update struct {
			id    int64
			score float64
		}
		var updates []update
		for rows.Next() {
			var id int64
			var in Inputs
			var knewCos float64
			var concepts []int64
			if err := rows.Scan(&id, &in.Relevance, &in.Excluded, &in.Significance, &in.Learning, &in.Sensational, &in.Publishers, &in.LastItemAt, &knewCos, &concepts); err != nil {
				rows.Close()
				return err
			}
			mastered := 0
			for _, c := range concepts {
				if states[c] == learning.StateMastered {
					mastered++
				}
			}
			share := 0.0
			if len(concepts) > 0 {
				share = float64(mastered) / float64(len(concepts))
			}
			in.Novelty = clamp01(knewCos + share)
			updates = append(updates, update{id, Score(in, r.tuning, now)})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, u := range updates {
			if _, err := tx.Exec(ctx, `UPDATE feed_entries SET score = $2, updated_at = now() WHERE id = $1`, u.id, u.score); err != nil {
				return err
			}
		}
		return nil
	})
}

// deliver moves the best candidates into the feed of day, the reader's local
// date, until the day's budget is full. Later stories that score well wait
// under "More". Done entries of the day still count.
func (r *Ranker) deliver(ctx context.Context, userID uuid.UUID, budget int, day string, now time.Time) (int64, error) {
	var n int64
	err := r.engine.WithTx(ctx, func(ctx context.Context) (err error) {
		n, err = drel.MustFromContext(ctx).Exec(ctx, `
			UPDATE feed_entries SET state = 'delivered', delivered_on = $2::date, delivered_at = now(), shown_revision = s.revision, updated_at = now()
			FROM stories s
			WHERE feed_entries.story_id = s.id AND feed_entries.id IN (
				SELECT f.id FROM feed_entries f JOIN stories st ON st.id = f.story_id
				WHERE f.user_id = $1 AND f.state = 'candidate' AND st.last_item_at >= $4 AND f.score > 0
				ORDER BY f.score DESC, f.id
				LIMIT GREATEST(0, $3 - (SELECT count(*) FROM feed_entries WHERE user_id = $1 AND delivered_on = $2::date))
			)`, userID, day, budget, now.Add(-r.tuning.Cluster.OpenWindow))
		return err
	})
	return n, err
}
