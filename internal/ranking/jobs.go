package ranking

import (
	"context"
	"time"

	"github.com/alternayte/drel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// RankArgs ranks the feed of one reader.
type RankArgs struct {
	UserID uuid.UUID `json:"user_id"`
}

func (RankArgs) Kind() string { return "ranking.rank" }

func (RankArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
		rivertype.JobStateRetryable, rivertype.JobStateScheduled,
	}}}
}

type RankWorker struct {
	river.WorkerDefaults[RankArgs]
	Ranker *Ranker
	// OnRelevant gets the stories that cleared the reader's prefilter.
	OnRelevant func(ctx context.Context, storyIDs []int64) error
	// OnRanked runs after every successful rank.
	OnRanked func(ctx context.Context) error
}

func (w *RankWorker) Timeout(*river.Job[RankArgs]) time.Duration { return 10 * time.Minute }

func (w *RankWorker) Work(ctx context.Context, job *river.Job[RankArgs]) error {
	res, err := w.Ranker.Rank(ctx, job.Args.UserID)
	if len(res.Relevant) > 0 && w.OnRelevant != nil {
		if nextErr := w.OnRelevant(ctx, res.Relevant); nextErr != nil {
			return nextErr
		}
	}
	if err != nil || w.OnRanked == nil {
		return err
	}
	return w.OnRanked(ctx)
}

// RankAllArgs ranks the feed of every onboarded reader.
type RankAllArgs struct{}

func (RankAllArgs) Kind() string { return "ranking.rank_all" }

func (RankAllArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateScheduled,
	}}}
}

type RankAllWorker struct {
	river.WorkerDefaults[RankAllArgs]
	Engine *drel.Engine
}

func (w *RankAllWorker) Work(ctx context.Context, _ *river.Job[RankAllArgs]) error {
	var users []uuid.UUID
	err := w.Engine.WithTx(ctx, func(ctx context.Context) error {
		rows, err := drel.MustFromContext(ctx).Query(ctx, `SELECT id FROM profiles WHERE onboarded_at IS NOT NULL ORDER BY id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return err
			}
			users = append(users, id)
		}
		return rows.Err()
	})
	if err != nil || len(users) == 0 {
		return err
	}
	params := make([]river.InsertManyParams, len(users))
	for i, u := range users {
		params[i] = river.InsertManyParams{Args: RankArgs{UserID: u}}
	}
	_, err = river.ClientFromContext[pgx.Tx](ctx).InsertMany(ctx, params)
	return err
}

// RankAllHourly is the periodic ranking of every reader.
func RankAllHourly() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return RankAllArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true})
}
