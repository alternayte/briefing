package learning

import (
	"testing"

	"github.com/alternayte/briefing/internal/platform"
)

func TestNextIntervalFollowsTheLadder(t *testing.T) {
	ladder := platform.DefaultTuning().Review.IntervalsDays
	steps := []struct {
		current int
		correct bool
		want    int
	}{
		{1, true, 3}, {3, true, 7}, {7, true, 16}, {16, true, 35}, {35, true, 35},
		{16, false, 1}, {1, false, 1},
	}
	for _, s := range steps {
		if got := nextInterval(s.current, s.correct, ladder); got != s.want {
			t.Errorf("from %d correct=%v: %d, want %d", s.current, s.correct, got, s.want)
		}
	}
}
