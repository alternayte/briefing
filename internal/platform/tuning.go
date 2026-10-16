package platform

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"time"
)

// CategoryOther is the triage category that drops an item.
const CategoryOther = "other"

// Tuning holds every threshold, weight, window and limit of the pipeline,
// ranking and learning. Changing a value is a code change with a test.
type Tuning struct {
	Poll     PollTuning
	Filter   FilterTuning
	Triage   TriageTuning
	Cluster  ClusterTuning
	Fetch    FetchTuning
	Write    WriteTuning
	Rank     RankTuning
	Score    ScoreWeights
	Penalty  PenaltyWeights
	Review   ReviewTuning
	Learning LearningTuning
	// Taxonomy lists the triage categories as stable slugs; the last one is
	// CategoryOther.
	Taxonomy []string
	// TextRetention is how long extracted article text is kept.
	TextRetention time.Duration
}

// PollTuning is the default poll interval of each source kind; a catalog
// entry can override it.
type PollTuning struct {
	RSS     time.Duration
	HN      time.Duration
	Bluesky time.Duration
	// Timeout and MaxBytes bound one poll request.
	Timeout  time.Duration
	MaxBytes int64
	// SnippetChars caps the feed text kept as an item snippet.
	SnippetChars int
}

type FilterTuning struct {
	MaxItemAge time.Duration
	// BlockedDomains are registrable domains whose items never pass.
	BlockedDomains []string
	// DenyPatterns are case-insensitive regular expressions on the title.
	DenyPatterns []string
}

type TriageTuning struct {
	// WorthMin is the lowest worth answer that keeps an item.
	WorthMin float64
	// MaxStateChars caps the state sent to any decision model.
	MaxStateChars int
}

type ClusterTuning struct {
	JoinHigh float64
	JoinLow  float64
	// SameEventMin is the lowest same_event answer that attaches an item in
	// the band between JoinLow and JoinHigh.
	SameEventMin float64
	// OpenWindow is how long a story accepts items after its last one.
	OpenWindow time.Duration
}

type FetchTuning struct {
	Timeout      time.Duration
	MaxBytes     int64
	MaxRedirects int
	// RenderBelowChars triggers rendering when readability returns less.
	RenderBelowChars int
}

type WriteTuning struct {
	// EvidenceFloorChars is the text length at least one item needs before
	// a story gets a summary.
	EvidenceFloorChars int
	SummaryMinWords    int
	SummaryMaxWords    int
	// SummaryRejectWords fails validation; it leaves slack above the prompt.
	SummaryRejectWords  int
	MinConcepts         int
	MaxConcepts         int
	ReviewQuestions     int
	ReviewChoices       int
	MaxRevisionsPerDay  int
	SynthesisPublishers float64
}

type RankTuning struct {
	// RelevanceMin is the lowest centroid-to-interest cosine that sends a
	// story to the relevance decision.
	RelevanceMin     float64
	Interval         time.Duration
	PublisherCap     int
	FreshnessHalf    time.Duration
	KnewWindow       time.Duration
	DefaultDailyFeed int
}

// ScoreWeights are the positive terms of the ranking score; they sum to 1.
type ScoreWeights struct {
	Relevance    float64
	Significance float64
	Learning     float64
	Coverage     float64
	Freshness    float64
}

type PenaltyWeights struct {
	Sensational float64
	Excluded    float64
	Novelty     float64
}

type ReviewTuning struct {
	// IntervalsDays is the spaced review ladder; a wrong answer goes back to
	// the first step.
	IntervalsDays []int
	// MaxDueCard caps the items in the Review card of one day.
	MaxDueCard int
}

type LearningTuning struct {
	MasteredCorrectDays int
	MasteredKnewStories int
	CalibrationStories  int
}

// DefaultTuning returns the tuning the app runs with.
func DefaultTuning() Tuning {
	return Tuning{
		Poll: PollTuning{
			RSS:          30 * time.Minute,
			HN:           10 * time.Minute,
			Bluesky:      15 * time.Minute,
			Timeout:      15 * time.Second,
			MaxBytes:     5 << 20,
			SnippetChars: 600,
		},
		Filter: FilterTuning{
			MaxItemAge:     72 * time.Hour,
			BlockedDomains: []string{},
			DenyPatterns: []string{
				`\bsponsored\b`,
				`\b(coupon|promo) codes?\b`,
				`\bbest\b.*\bdeals?\b`,
				`\b\d+% off\b`,
				`\b(we'?re|now) hiring\b`,
			},
		},
		Triage: TriageTuning{WorthMin: 0.5, MaxStateChars: 6000},
		Cluster: ClusterTuning{
			JoinHigh:     0.85,
			JoinLow:      0.72,
			SameEventMin: 0.6,
			OpenWindow:   72 * time.Hour,
		},
		Fetch: FetchTuning{
			Timeout:          10 * time.Second,
			MaxBytes:         2 << 20,
			MaxRedirects:     3,
			RenderBelowChars: 800,
		},
		Write: WriteTuning{
			EvidenceFloorChars:  1500,
			SummaryMinWords:     40,
			SummaryMaxWords:     100,
			SummaryRejectWords:  120,
			MinConcepts:         2,
			MaxConcepts:         5,
			ReviewQuestions:     2,
			ReviewChoices:       4,
			MaxRevisionsPerDay:  3,
			SynthesisPublishers: 2,
		},
		Rank: RankTuning{
			RelevanceMin:     0.30,
			Interval:         time.Hour,
			PublisherCap:     4,
			FreshnessHalf:    24 * time.Hour,
			KnewWindow:       30 * 24 * time.Hour,
			DefaultDailyFeed: 10,
		},
		Score: ScoreWeights{
			Relevance:    0.40,
			Significance: 0.20,
			Learning:     0.15,
			Coverage:     0.10,
			Freshness:    0.15,
		},
		Penalty: PenaltyWeights{Sensational: 0.15, Excluded: 1.0, Novelty: 0.30},
		Review:  ReviewTuning{IntervalsDays: []int{1, 3, 7, 16, 35}, MaxDueCard: 5},
		Learning: LearningTuning{
			MasteredCorrectDays: 3,
			MasteredKnewStories: 3,
			CalibrationStories:  10,
		},
		Taxonomy: []string{
			"ai_ml",
			"software_engineering",
			"programming_languages",
			"security",
			"infrastructure_cloud",
			"hardware_chips",
			"space",
			"physics",
			"biology_medicine",
			"climate_energy",
			"mathematics",
			"science_policy",
			CategoryOther,
		},
		TextRetention: 30 * 24 * time.Hour,
	}
}

// Validate checks the invariants that a tuning change can break.
func (t Tuning) Validate() error {
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	s := t.Score
	if sum := s.Relevance + s.Significance + s.Learning + s.Coverage + s.Freshness; math.Abs(sum-1) > 1e-9 {
		fail("score weights sum to %v, want 1", sum)
	}
	if !(0 < t.Cluster.JoinLow && t.Cluster.JoinLow < t.Cluster.JoinHigh && t.Cluster.JoinHigh <= 1) {
		fail("cluster join band must satisfy 0 < low < high <= 1, got %v..%v", t.Cluster.JoinLow, t.Cluster.JoinHigh)
	}
	iv := t.Review.IntervalsDays
	if len(iv) == 0 || iv[0] < 1 || !slices.IsSorted(iv) || len(slices.Compact(slices.Clone(iv))) != len(iv) {
		fail("review intervals must rise strictly from at least 1 day, got %v", iv)
	}
	if len(t.Taxonomy) == 0 || t.Taxonomy[len(t.Taxonomy)-1] != CategoryOther {
		fail("taxonomy must end with %q", CategoryOther)
	}
	w := t.Write
	if !(0 < w.MinConcepts && w.MinConcepts <= w.MaxConcepts) {
		fail("concepts range must satisfy 0 < min <= max, got %d..%d", w.MinConcepts, w.MaxConcepts)
	}
	if !(w.SummaryMinWords < w.SummaryMaxWords && w.SummaryMaxWords <= w.SummaryRejectWords) {
		fail("summary words must satisfy min < max <= reject")
	}
	for _, p := range t.Filter.DenyPatterns {
		if _, err := regexp.Compile("(?i)" + p); err != nil {
			fail("deny pattern %q: %v", p, err)
		}
	}
	if min(t.Poll.RSS, t.Poll.HN, t.Poll.Bluesky) < time.Minute {
		fail("poll intervals must be at least one minute, got %+v", t.Poll)
	}
	if len(errs) > 0 {
		return fmt.Errorf("tuning: %w", errors.Join(errs...))
	}
	return nil
}
