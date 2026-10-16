package platform

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/alternayte/drel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

const (
	// QueuePoll keeps source polls apart from the pipeline, so a slow feed
	// never holds up triage or writing.
	QueuePoll = "poll"
	// QueueCluster has one worker: clustering reads the open stories and
	// then writes, so two runs at once could open two stories for one event.
	QueueCluster = "cluster"
	// QueueFetch holds article fetches, which wait on other sites for
	// seconds each, so they never take the workers of the model steps.
	QueueFetch = "fetch"
)

// Next is the hook a pipeline step calls inside its write transaction with
// the ids for the following step. A nil Next ends the pipeline there.
type Next func(ctx context.Context, ids []int64) error

// InsertInTx inserts jobs in the drel transaction of ctx, so they commit or
// roll back with the writes of the step. ctx must come from a River worker.
func InsertInTx(ctx context.Context, params ...river.InsertManyParams) error {
	tx := drel.MustFromContext(ctx)
	if err := tx.SaveChanges(ctx); err != nil {
		return err
	}
	_, err := river.ClientFromContext[pgx.Tx](ctx).InsertManyTx(ctx, tx.Underlying().(pgx.Tx), params)
	return err
}

// NewRiver returns a River client. With nil workers it only inserts jobs.
func NewRiver(pool *pgxpool.Pool, workers *river.Workers, periodic []*river.PeriodicJob) (*river.Client[pgx.Tx], error) {
	cfg := &river.Config{Logger: slog.Default()}
	if workers != nil {
		cfg.Workers = workers
		cfg.PeriodicJobs = periodic
		cfg.Queues = map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 10},
			QueuePoll:          {MaxWorkers: 8},
			QueueCluster:       {MaxWorkers: 1},
			QueueFetch:         {MaxWorkers: 6},
		}
	}
	c, err := river.NewClient(riverpgxv5.New(pool), cfg)
	if err != nil {
		return nil, fmt.Errorf("river client: %w", err)
	}
	return c, nil
}
