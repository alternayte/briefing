package stories

import (
	"context"
	"time"

	"github.com/alternayte/drel"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/alternayte/briefing/internal/platform"
)

// FilterArgs runs the rule filter on items a poll touched.
type FilterArgs struct {
	ItemIDs []int64 `json:"item_ids"`
}

func (FilterArgs) Kind() string { return "stories.filter" }

// FilterWorker applies the rules and hands the items that passed to the next
// step, when one is set.
type FilterWorker struct {
	river.WorkerDefaults[FilterArgs]
	Filter   *Filter
	OnPassed func(ctx context.Context, itemIDs []int64) error
}

func (w *FilterWorker) Work(ctx context.Context, job *river.Job[FilterArgs]) error {
	passed, err := w.Filter.Apply(ctx, job.Args.ItemIDs)
	if err != nil || len(passed) == 0 || w.OnPassed == nil {
		return err
	}
	return w.OnPassed(ctx, passed)
}

// ClusterArgs places embedded items into stories. It runs on the one-worker
// cluster queue.
type ClusterArgs struct {
	ItemIDs []int64 `json:"item_ids"`
}

func (ClusterArgs) Kind() string { return "stories.cluster" }

func (ClusterArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: platform.QueueCluster}
}

type ClusterWorker struct {
	river.WorkerDefaults[ClusterArgs]
	Clusterer   *Clusterer
	OnClustered func(ctx context.Context, storyIDs []int64) error
}

func (w *ClusterWorker) Work(ctx context.Context, job *river.Job[ClusterArgs]) error {
	stories, err := w.Clusterer.Cluster(ctx, job.Args.ItemIDs)
	if len(stories) > 0 && w.OnClustered != nil {
		if nextErr := w.OnClustered(ctx, stories); nextErr != nil {
			return nextErr
		}
	}
	return err
}

// CloseStaleArgs closes stories that received no item for the open window.
type CloseStaleArgs struct{}

func (CloseStaleArgs) Kind() string { return "stories.close_stale" }

type CloseStaleWorker struct {
	river.WorkerDefaults[CloseStaleArgs]
	Clusterer *Clusterer
}

func (w *CloseStaleWorker) Work(ctx context.Context, _ *river.Job[CloseStaleArgs]) error {
	_, err := w.Clusterer.CloseStale(ctx)
	return err
}

// CloseStaleHourly is the periodic job that closes quiet stories.
func CloseStaleHourly() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return CloseStaleArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true})
}

// TriageArgs asks the decision model about items that passed the rules.
type TriageArgs struct {
	ItemIDs []int64 `json:"item_ids"`
}

func (TriageArgs) Kind() string { return "stories.triage" }

type TriageWorker struct {
	river.WorkerDefaults[TriageArgs]
	Triager *Triager
	OnKept  func(ctx context.Context, itemIDs []int64) error
}

func (w *TriageWorker) Work(ctx context.Context, job *river.Job[TriageArgs]) error {
	kept, err := w.Triager.Triage(ctx, job.Args.ItemIDs)
	// Hand on what was kept even when some calls failed; the retry asks only
	// about the rest.
	if len(kept) > 0 && w.OnKept != nil {
		if nextErr := w.OnKept(ctx, kept); nextErr != nil {
			return nextErr
		}
	}
	return err
}

// EmbedArgs embeds kept items.
type EmbedArgs struct {
	ItemIDs []int64 `json:"item_ids"`
}

func (EmbedArgs) Kind() string { return "stories.embed" }

type EmbedWorker struct {
	river.WorkerDefaults[EmbedArgs]
	Embedder   *ItemEmbedder
	OnEmbedded func(ctx context.Context, itemIDs []int64) error
}

func (w *EmbedWorker) Work(ctx context.Context, job *river.Job[EmbedArgs]) error {
	done, err := w.Embedder.Embed(ctx, job.Args.ItemIDs)
	if err != nil || len(done) == 0 || w.OnEmbedded == nil {
		return err
	}
	return w.OnEmbedded(ctx, done)
}

// SweepArgs hands stalled items on to their next step.
type SweepArgs struct{}

func (SweepArgs) Kind() string { return "stories.sweep" }

type SweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	Engine *drel.Engine
	Now    func() time.Time
}

const (
	sweepGrace = 5 * time.Minute
	sweepBatch = 200
)

func (w *SweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	s, err := FindStalled(ctx, w.Engine, w.Now(), sweepGrace, sweepBatch)
	if err != nil {
		return err
	}
	var params []river.InsertManyParams
	add := func(ids []int64, args func([]int64) river.JobArgs) {
		if len(ids) > 0 {
			params = append(params, river.InsertManyParams{Args: args(ids)})
		}
	}
	add(s.New, func(ids []int64) river.JobArgs { return FilterArgs{ItemIDs: ids} })
	add(s.Passed, func(ids []int64) river.JobArgs { return TriageArgs{ItemIDs: ids} })
	add(s.Kept, func(ids []int64) river.JobArgs { return EmbedArgs{ItemIDs: ids} })
	add(s.Unplaced, func(ids []int64) river.JobArgs { return ClusterArgs{ItemIDs: ids} })
	if len(params) == 0 {
		return nil
	}
	_, err = river.ClientFromContext[pgx.Tx](ctx).InsertMany(ctx, params)
	return err
}

// SweepEveryFiveMinutes is the periodic job of the pipeline sweep.
func SweepEveryFiveMinutes() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(5*time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true})
}
