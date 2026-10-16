package stories

import (
	"context"
	"time"

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
