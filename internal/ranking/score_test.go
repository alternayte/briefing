package ranking

import (
	"math"
	"testing"
	"time"

	"github.com/alternayte/briefing/internal/platform"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestScoreFollowsTheFormula(t *testing.T) {
	tu := platform.DefaultTuning()
	now := time.Date(2031, 6, 1, 12, 0, 0, 0, time.UTC)
	in := Inputs{
		Relevance:    3,                        // core interest → 1
		Significance: 2,                        // notable → 0.5
		Learning:     1.5,                      // → 0.5
		Publishers:   2,                        // → 0.5
		LastItemAt:   now.Add(-24 * time.Hour), // one half-life → 0.5
		Sensational:  0.2,
		Excluded:     0.1,
		Novelty:      0.5,
	}
	want := 0.40*1 + 0.20*0.5 + 0.15*0.5 + 0.10*0.5 + 0.15*0.5 - (0.15*0.2 + 1.0*0.1 + 0.30*0.5)
	if got := Score(in, tu, now); !near(got, want) {
		t.Errorf("score = %v, want %v", got, want)
	}
}

func TestPublisherCoverageCapsAtFour(t *testing.T) {
	tu := platform.DefaultTuning()
	now := time.Now()
	four := Score(Inputs{Publishers: 4, LastItemAt: now}, tu, now)
	nine := Score(Inputs{Publishers: 9, LastItemAt: now}, tu, now)
	if !near(four, nine) {
		t.Errorf("4 publishers %v, 9 publishers %v", four, nine)
	}
}

func TestExcludedStoryFallsBelowEverything(t *testing.T) {
	tu := platform.DefaultTuning()
	now := time.Now()
	best := Inputs{Relevance: 3, Significance: 4, Learning: 3, Publishers: 4, LastItemAt: now}
	excluded := best
	excluded.Excluded = 1
	worst := Inputs{LastItemAt: now.Add(-30 * 24 * time.Hour)}
	if Score(excluded, tu, now) >= Score(worst, tu, now) {
		t.Errorf("an excluded story outranks an irrelevant old one")
	}
}
