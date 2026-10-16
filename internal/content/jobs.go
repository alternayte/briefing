package content

import (
	"context"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/alternayte/briefing/internal/platform"
)

// FetchArgs fills the text of the items of one shortlisted story.
type FetchArgs struct {
	StoryID int64 `json:"story_id"`
}

func (FetchArgs) Kind() string { return "content.fetch" }

func (FetchArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: platform.QueueFetch, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
		rivertype.JobStateRetryable, rivertype.JobStateScheduled,
	}}}
}

type FetchWorker struct {
	river.WorkerDefaults[FetchArgs]
	Texts     *Texts
	OnFetched func(ctx context.Context, storyID int64) error
}

// Timeout allows each item of a large story its full fetch time.
func (w *FetchWorker) Timeout(*river.Job[FetchArgs]) time.Duration { return 10 * time.Minute }

func (w *FetchWorker) Work(ctx context.Context, job *river.Job[FetchArgs]) error {
	if _, err := w.Texts.FillStory(ctx, job.Args.StoryID); err != nil {
		return err
	}
	if w.OnFetched == nil {
		return nil
	}
	return w.OnFetched(ctx, job.Args.StoryID)
}
