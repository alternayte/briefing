package write

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/alternayte/drel"

	"github.com/alternayte/briefing/internal/embed"
	"github.com/alternayte/briefing/internal/ingest"
	"github.com/alternayte/briefing/internal/platform"
)

// Writer writes the English entry of a story.
type Writer struct {
	engine   *drel.Engine
	or       *platform.OpenRouter
	embedder *embed.Embedder
	model    string
	tuning   platform.WriteTuning
	capUSD   float64
	now      func() time.Time
}

func NewWriter(engine *drel.Engine, or *platform.OpenRouter, e *embed.Embedder, cfg platform.Config, t platform.Tuning, now func() time.Time) *Writer {
	return &Writer{engine: engine, or: or, embedder: e, model: cfg.WriteModel, tuning: t.Write, capUSD: cfg.DailySpendUSD, now: now}
}

// story is what the writer reads about a story.
type story struct {
	publishers int
	revision   int
	sources    []Source
	textItems  int
	longest    int
	// last is the newest summary's publisher count; 0 when none exists.
	lastPublishers int
	writtenToday   int
	attemptsToday  int
}

// Write writes a new revision of the story's entry when the story needs one
// and the rules allow it. It reports whether it wrote.
func (w *Writer) Write(ctx context.Context, storyID int64) (bool, error) {
	now := w.now().UTC()
	st, err := w.load(ctx, storyID, now)
	if err != nil {
		return false, err
	}
	switch {
	case st.longest < w.tuning.EvidenceFloorChars:
		return false, nil // below the evidence floor the story is a link card
	case st.lastPublishers > 0 && float64(st.publishers) < w.tuning.SynthesisPublishers*float64(st.lastPublishers):
		return false, nil // the coverage has not grown enough for a new revision
	case st.writtenToday >= w.tuning.MaxRevisionsPerDay, st.attemptsToday >= w.tuning.MaxRevisionsPerDay:
		return false, nil
	}

	supplied := map[string]bool{}
	for _, s := range st.sources {
		supplied[s.ID] = true
	}
	var out Output
	var previous, problems string
	for attempt := 0; ; attempt++ {
		if st.attemptsToday+attempt >= w.tuning.MaxRevisionsPerDay {
			return false, nil
		}
		if ok, err := w.underCap(ctx, now); err != nil || !ok {
			return false, err
		}
		if err := w.countAttempt(ctx, storyID, now); err != nil {
			return false, err
		}
		var content string
		content, out, err = w.call(ctx, st.sources, previous, problems)
		if err == nil {
			err = validate(out, supplied, w.tuning)
		}
		previous = content
		if err == nil {
			break
		}
		var apiErr *platform.APIError
		if errors.As(err, &apiErr) {
			return false, err // the job retries a provider failure
		}
		if attempt == 1 {
			slog.WarnContext(ctx, "writer output rejected twice", "story", storyID, "error", err)
			return false, nil
		}
		problems = err.Error()
	}

	concepts, err := w.matchConcepts(ctx, out.Concepts)
	if err != nil {
		return false, err
	}
	return true, w.store(ctx, storyID, st, out, concepts)
}

func (w *Writer) load(ctx context.Context, storyID int64, now time.Time) (story, error) {
	var st story
	day := now.Format(time.DateOnly)
	err := w.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		if err := tx.QueryRow(ctx, `SELECT publisher_count, revision FROM stories WHERE id = $1`, storyID).Scan(&st.publishers, &st.revision); err != nil {
			return err
		}
		err := tx.QueryRow(ctx, `
			SELECT
				COALESCE((SELECT publisher_count FROM summaries WHERE story_id = $1 ORDER BY revision DESC LIMIT 1), 0),
				(SELECT count(*) FROM summaries WHERE story_id = $1 AND created_at::date = $2::date),
				COALESCE((SELECT attempts FROM write_attempts WHERE story_id = $1 AND day = $2::date), 0)`,
			storyID, day).Scan(&st.lastPublishers, &st.writtenToday, &st.attemptsToday)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT i.id, i.canonical_url, i.title, i.text, i.text_len
			FROM story_items si JOIN items i ON i.id = si.item_id
			WHERE si.story_id = $1 AND i.text_len > 0
			ORDER BY i.text_len DESC, i.id`, storyID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			var url, title, text string
			var n int
			if err := rows.Scan(&id, &url, &title, &text, &n); err != nil {
				return err
			}
			st.textItems++
			st.longest = max(st.longest, n)
			if len(st.sources) < w.tuning.MaxSources {
				st.sources = append(st.sources, Source{ID: strconv.FormatInt(id, 10), Publisher: ingest.RegistrableDomain(url), Title: title, Text: text})
			}
		}
		return rows.Err()
	})
	if err != nil {
		return st, fmt.Errorf("load story %d: %w", storyID, err)
	}
	return st, nil
}

func (w *Writer) underCap(ctx context.Context, now time.Time) (bool, error) {
	spent, err := w.or.SpentOn(ctx, now)
	if err != nil {
		return false, err
	}
	return spent < w.capUSD, nil
}

func (w *Writer) countAttempt(ctx context.Context, storyID int64, now time.Time) error {
	return w.engine.WithTx(ctx, func(ctx context.Context) error {
		_, err := drel.MustFromContext(ctx).Exec(ctx, `
			INSERT INTO write_attempts (story_id, day, attempts) VALUES ($1, $2::date, 1)
			ON CONFLICT (story_id, day) DO UPDATE SET attempts = write_attempts.attempts + 1, updated_at = now()`,
			storyID, now.Format(time.DateOnly))
		return err
	})
}

// call asks the writer model. After a rejected answer, previous and problems
// carry that answer and the rules it broke, so the model can fix it.
func (w *Writer) call(ctx context.Context, sources []Source, previous, problems string) (string, Output, error) {
	messages := []platform.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt(sources, w.tuning.MaxCharsPerSource)},
	}
	if problems != "" && previous != "" {
		messages = append(messages,
			platform.ChatMessage{Role: "assistant", Content: previous},
			platform.ChatMessage{Role: "user", Content: "Your answer broke these rules:\n" + problems + "\nAnswer again with the corrected JSON object only."})
	}
	content, err := w.or.Chat(ctx, w.model, "write", messages, w.tuning.MaxOutputTokens, true)
	if err != nil {
		return content, Output{}, err
	}
	out, err := parseOutput(content)
	return content, out, err
}

// conceptRef is a concept of the new entry, matched to an existing row or
// to be inserted.
type conceptRef struct {
	OutConcept
	slug      string
	id        int64
	embedding platform.Vector
}

// matchConcepts finds each concept by slug, then by the nearest embedding;
// the rest get an embedding for insertion.
func (w *Writer) matchConcepts(ctx context.Context, cs []OutConcept) ([]conceptRef, error) {
	refs := make([]conceptRef, 0, len(cs))
	seen := map[string]bool{}
	for _, c := range cs {
		if s := slug(c.Name); !seen[s] {
			seen[s] = true
			refs = append(refs, conceptRef{OutConcept: c, slug: s})
		}
	}
	err := w.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		for i := range refs {
			err := tx.QueryRow(ctx, `SELECT id FROM concepts WHERE slug = $1`, refs[i].slug).Scan(&refs[i].id)
			if err != nil && !isNoRows(err) {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var texts []string
	var unmatched []int
	for i, r := range refs {
		if r.id == 0 {
			texts = append(texts, r.Name+": "+r.Definition)
			unmatched = append(unmatched, i)
		}
	}
	if len(texts) == 0 {
		return refs, nil
	}
	vectors, err := w.embedder.Embed(ctx, "concept_embed", texts)
	if err != nil {
		return nil, err
	}
	err = w.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		for k, i := range unmatched {
			refs[i].embedding = vectors[k]
			v, err := vectors[k].Value()
			if err != nil {
				return err
			}
			var id int64
			var sim float64
			err = tx.QueryRow(ctx, `
				SELECT id, 1 - (embedding <=> $1::vector) FROM concepts
				WHERE embed_model = $2 AND vector_dims(embedding) = vector_dims($1::vector)
				ORDER BY embedding <=> $1::vector LIMIT 1`, v, w.embedder.Model()).Scan(&id, &sim)
			if err != nil && !isNoRows(err) {
				return err
			}
			if err == nil && sim >= w.tuning.ConceptMatchMin {
				refs[i].id = id
			}
		}
		return nil
	})
	return refs, err
}

func (w *Writer) store(ctx context.Context, storyID int64, st story, out Output, concepts []conceptRef) error {
	revision := st.revision + 1
	claims := make(Claims, 0, len(out.Claims))
	for _, c := range out.Claims {
		ids := make([]int64, 0, len(c.ItemIDs))
		for _, s := range c.ItemIDs {
			id, _ := strconv.ParseInt(s, 10, 64)
			ids = append(ids, id)
		}
		claims = append(claims, Claim{Text: c.Text, ItemIDs: ids})
	}
	err := w.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		drel.NewTxRepository(tx, SummaryMeta).Add(&Summary{
			StoryID: storyID, Revision: revision,
			Headline: out.Headline, Summary: out.Summary, WhyItMatters: out.WhyItMatters, WhatIsNew: out.WhatIsNew,
			Claims: claims, Confidence: confidence(st.publishers, st.textItems),
			WriterModel: w.model, PromptVersion: PromptVersion,
			PublisherCount: st.publishers, TextItems: st.textItems,
		})
		reviews := drel.NewTxRepository(tx, ReviewItemMeta)
		for _, r := range out.Review {
			reviews.Add(&ReviewItem{StoryID: storyID, Revision: revision, Question: r.Question, Choices: r.Choices, AnswerIndex: r.AnswerIndex, Explanation: r.Explanation})
		}
		if err := tx.SaveChanges(ctx); err != nil {
			return err
		}
		for _, c := range concepts {
			id := c.id
			if id == 0 {
				v, err := c.embedding.Value()
				if err != nil {
					return err
				}
				err = tx.QueryRow(ctx, `
					INSERT INTO concepts (slug, name, definition, embedding, embed_model) VALUES ($1, $2, $3, $4::vector, $5)
					ON CONFLICT (slug) DO UPDATE SET updated_at = now() RETURNING id`,
					c.slug, c.Name, c.Definition, v, w.embedder.Model()).Scan(&id)
				if err != nil {
					return err
				}
			}
			if _, err := tx.Exec(ctx, `INSERT INTO story_concepts (story_id, concept_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, storyID, id); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `UPDATE stories SET revision = $2, updated_at = now() WHERE id = $1`, storyID, revision)
		return err
	})
	if err != nil {
		return fmt.Errorf("store entry of story %d: %w", storyID, err)
	}
	return nil
}
