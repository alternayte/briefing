// Package embed turns text into vectors through OpenRouter.
package embed

import (
	"context"
	"fmt"

	"github.com/alternayte/briefing/internal/platform"
)

const batchSize = 64

type Embedder struct {
	or    *platform.OpenRouter
	model string
	dim   int
}

func New(or *platform.OpenRouter, cfg platform.Config) *Embedder {
	return &Embedder{or: or, model: cfg.EmbedModel, dim: cfg.EmbedDim}
}

// Model is stored with every vector, so a model change never mixes spaces.
func (e *Embedder) Model() string { return e.model }

// Embed returns one vector per text, in input order.
func (e *Embedder) Embed(ctx context.Context, stage string, texts []string) ([]platform.Vector, error) {
	out := make([]platform.Vector, 0, len(texts))
	for start := 0; start < len(texts); start += batchSize {
		batch := texts[start:min(start+batchSize, len(texts))]
		var resp struct {
			Data []struct {
				Index     int       `json:"index"`
				Embedding []float32 `json:"embedding"`
			} `json:"data"`
		}
		req := map[string]any{"model": e.model, "input": batch}
		if err := e.or.Post(ctx, "/api/v1/embeddings", stage, e.model, req, &resp); err != nil {
			return nil, fmt.Errorf("embed: %w", err)
		}
		if len(resp.Data) != len(batch) {
			return nil, fmt.Errorf("embed: %d vectors for %d texts", len(resp.Data), len(batch))
		}
		ordered := make([]platform.Vector, len(batch))
		for _, d := range resp.Data {
			if d.Index < 0 || d.Index >= len(batch) {
				return nil, fmt.Errorf("embed: index %d out of range", d.Index)
			}
			if len(d.Embedding) != e.dim {
				return nil, fmt.Errorf("embed: got %d dimensions, EMBED_DIM is %d", len(d.Embedding), e.dim)
			}
			ordered[d.Index] = platform.NewVector(d.Embedding)
		}
		out = append(out, ordered...)
	}
	return out, nil
}
