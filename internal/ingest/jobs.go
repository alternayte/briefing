package ingest

import (
	"context"
	"fmt"
	"time"

	"github.com/alternayte/drel"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/sources"
)

// onePerSource keeps at most one queued or running poll for each source.
var onePerSource = river.UniqueOpts{
	ByArgs: true,
	ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
		rivertype.JobStateRetryable, rivertype.JobStateScheduled,
	},
}

// PollArgs polls one source.
type PollArgs struct {
	SourceID int64 `json:"source_id"`
}

func (PollArgs) Kind() string { return "ingest.poll" }

func (PollArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: platform.QueuePoll, MaxAttempts: 3, UniqueOpts: onePerSource}
}

// PollWorker runs a poll and hands the items it touched to the next step.
type PollWorker struct {
	river.WorkerDefaults[PollArgs]
	Poller  *Poller
	OnItems func(ctx context.Context, itemIDs []int64) error
}

func (w *PollWorker) Timeout(*river.Job[PollArgs]) time.Duration { return 2 * time.Minute }

func (w *PollWorker) Work(ctx context.Context, job *river.Job[PollArgs]) error {
	ids, err := w.Poller.Poll(ctx, job.Args.SourceID)
	if err != nil || len(ids) == 0 {
		return err
	}
	return w.OnItems(ctx, ids)
}

// SweepArgs enqueues a poll for every due source. It runs every minute, so a
// lost enqueue or a crashed worker delays a poll by a minute at most.
type SweepArgs struct{}

func (SweepArgs) Kind() string { return "ingest.sweep" }

type SweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	Engine *drel.Engine
	Now    func() time.Time
}

func (w *SweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	ids, err := sources.Due(ctx, w.Engine, w.Now())
	if err != nil || len(ids) == 0 {
		return err
	}
	params := make([]river.InsertManyParams, len(ids))
	for i, id := range ids {
		params[i] = river.InsertManyParams{Args: PollArgs{SourceID: id}}
	}
	if _, err := river.ClientFromContext[pgx.Tx](ctx).InsertMany(ctx, params); err != nil {
		return fmt.Errorf("enqueue polls: %w", err)
	}
	return nil
}

// SweepEveryMinute is the periodic job that keeps polls flowing.
func SweepEveryMinute() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true})
}
