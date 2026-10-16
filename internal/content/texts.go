package content

import (
	"context"
	"fmt"
	"log/slog"
	"unicode/utf8"

	"github.com/alternayte/drel"

	"github.com/alternayte/briefing/internal/ingest"
	"github.com/alternayte/briefing/internal/platform"
)

// Texts fills the text of the items of a story.
type Texts struct {
	engine      *drel.Engine
	fetcher     *Fetcher
	renderer    *Renderer
	renderBelow int
}

// NewTexts builds the step. A nil renderer turns rendering off.
func NewTexts(engine *drel.Engine, f *Fetcher, r *Renderer, t platform.Tuning) *Texts {
	return &Texts{engine: engine, fetcher: f, renderer: r, renderBelow: t.Fetch.RenderBelowChars}
}

type pendingItem struct {
	id  int64
	url string
}

// FillStory fetches every item of the story that has no text yet. A page that
// fails stays without text, with text_source none, and does not stop the rest.
func (t *Texts) FillStory(ctx context.Context, storyID int64) (int, error) {
	var pending []pendingItem
	err := t.engine.WithTx(ctx, func(ctx context.Context) error {
		rows, err := drel.MustFromContext(ctx).Query(ctx, `
			SELECT i.id, i.canonical_url FROM story_items si JOIN items i ON i.id = si.item_id
			WHERE si.story_id = $1 AND i.text_source = 'pending' ORDER BY i.id`, storyID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p pendingItem
			if err := rows.Scan(&p.id, &p.url); err != nil {
				return err
			}
			pending = append(pending, p)
		}
		return rows.Err()
	})
	if err != nil {
		return 0, err
	}
	for _, p := range pending {
		text, src := t.text(ctx, p.url)
		err := t.engine.WithTx(ctx, func(ctx context.Context) error {
			_, err := drel.MustFromContext(ctx).Exec(ctx,
				`UPDATE items SET text = $2, text_len = $3, text_source = $4, updated_at = now() WHERE id = $1`,
				p.id, text, utf8.RuneCountInString(text), src)
			return err
		})
		if err != nil {
			return 0, fmt.Errorf("store text of item %d: %w", p.id, err)
		}
	}
	return len(pending), nil
}

func (t *Texts) text(ctx context.Context, pageURL string) (string, ingest.TextSource) {
	text, err := t.fetcher.FetchText(ctx, pageURL)
	if err != nil {
		slog.InfoContext(ctx, "fetch failed", "url", pageURL, "error", err)
	}
	if utf8.RuneCountInString(text) < t.renderBelow && t.renderer != nil {
		rendered, rerr := t.renderer.Render(ctx, pageURL)
		if rerr != nil {
			slog.InfoContext(ctx, "render failed", "url", pageURL, "error", rerr)
		} else if utf8.RuneCountInString(rendered) > utf8.RuneCountInString(text) {
			return rendered, ingest.TextRender
		}
	}
	if text == "" {
		return "", ingest.TextNone
	}
	return text, ingest.TextExtract
}
