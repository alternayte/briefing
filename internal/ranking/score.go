// Package ranking scores stories for each reader and fills their feed.
package ranking

import (
	"math"
	"time"

	"github.com/alternayte/briefing/internal/platform"
)

// Inputs are the facts about one story for one reader. Relevance,
// Significance and Learning are the decision models' level indexes;
// Sensational, Excluded and Novelty are in [0, 1].
type Inputs struct {
	Relevance    float64
	Significance float64
	Learning     float64
	Publishers   int
	LastItemAt   time.Time
	Sensational  float64
	Excluded     float64
	Novelty      float64
}

// Score combines the inputs with the tuning weights.
func Score(in Inputs, t platform.Tuning, now time.Time) float64 {
	w, p := t.Score, t.Penalty
	r := clamp01(in.Relevance / 3)
	s := clamp01(in.Significance / 4)
	k := clamp01(in.Learning / 3)
	c := float64(min(in.Publishers, t.Rank.PublisherCap)) / float64(t.Rank.PublisherCap)
	f := math.Pow(0.5, max(0, now.Sub(in.LastItemAt).Hours())/t.Rank.FreshnessHalf.Hours())
	positive := w.Relevance*r + w.Significance*s + w.Learning*k + w.Coverage*c + w.Freshness*f
	penalty := p.Sensational*clamp01(in.Sensational) + p.Excluded*clamp01(in.Excluded) + p.Novelty*clamp01(in.Novelty)
	return positive - penalty
}

func clamp01(x float64) float64 { return min(1, max(0, x)) }
