package platform

import (
	"fmt"
	"log/slog"

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
)

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
		}
	}
	c, err := river.NewClient(riverpgxv5.New(pool), cfg)
	if err != nil {
		return nil, fmt.Errorf("river client: %w", err)
	}
	return c, nil
}
