package content

import (
	"context"

	"github.com/riverqueue/river"
)

// FetchArgs fills the text of the items of one shortlisted story.
type FetchArgs struct {
	StoryID int64 `json:"story_id"`
}

func (FetchArgs) Kind() string { return "content.fetch" }

type FetchWorker struct {
	river.WorkerDefaults[FetchArgs]
	Texts     *Texts
	OnFetched func(ctx context.Context, storyID int64) error
}

func (w *FetchWorker) Work(ctx context.Context, job *river.Job[FetchArgs]) error {
	if _, err := w.Texts.FillStory(ctx, job.Args.StoryID); err != nil {
		return err
	}
	if w.OnFetched == nil {
		return nil
	}
	return w.OnFetched(ctx, job.Args.StoryID)
}
