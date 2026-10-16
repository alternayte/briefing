package write

import (
	"context"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// WriteArgs writes the entry of one story when it needs one.
type WriteArgs struct {
	StoryID int64 `json:"story_id"`
}

func (WriteArgs) Kind() string { return "write.story" }

func (WriteArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 5, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
		rivertype.JobStateRetryable, rivertype.JobStateScheduled,
	}}}
}

type WriteWorker struct {
	river.WorkerDefaults[WriteArgs]
	Writer    *Writer
	OnWritten func(ctx context.Context, storyID int64) error
}

func (w *WriteWorker) Timeout(*river.Job[WriteArgs]) time.Duration { return 5 * time.Minute }

func (w *WriteWorker) Work(ctx context.Context, job *river.Job[WriteArgs]) error {
	written, err := w.Writer.Write(ctx, job.Args.StoryID)
	if err != nil || !written || w.OnWritten == nil {
		return err
	}
	return w.OnWritten(ctx, job.Args.StoryID)
}
