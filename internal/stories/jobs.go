package stories

import (
	"context"

	"github.com/riverqueue/river"
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
