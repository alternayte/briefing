package translate

import (
	"context"
	"errors"
	"time"

	"github.com/alternayte/drel"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/alternayte/briefing/internal/i18n"
)

// TranslateArgs translates one story's entry into one locale.
type TranslateArgs struct {
	StoryID int64  `json:"story_id"`
	Locale  string `json:"locale"`
}

func (TranslateArgs) Kind() string { return "translate.story" }

func (TranslateArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 4, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
		rivertype.JobStateRetryable, rivertype.JobStateScheduled,
	}}}
}

type TranslateWorker struct {
	river.WorkerDefaults[TranslateArgs]
	Translator *Translator
}

func (w *TranslateWorker) Work(ctx context.Context, job *river.Job[TranslateArgs]) error {
	l, ok := i18n.Parse(job.Args.Locale)
	if !ok {
		return nil
	}
	err := w.Translator.TranslateStory(ctx, job.Args.StoryID, l)
	if errors.Is(err, ErrCapReached) {
		// The reader sees English today; the sweep retries after the reset.
		return nil
	}
	return err
}

// shownPerReader is how many waiting stories per reader count as shown,
// besides today's: the top of "More".
const shownPerReader = 10

// Missing returns the (story, locale) pairs that non-English readers are
// shown and that have no translation of the current revision yet.
func Missing(ctx context.Context, engine *drel.Engine, today time.Time) ([]TranslateArgs, error) {
	var out []TranslateArgs
	err := engine.WithTx(ctx, func(ctx context.Context) error {
		rows, err := drel.MustFromContext(ctx).Query(ctx, `
			WITH shown AS (
				SELECT f.story_id, p.locale, f.delivered_on, f.state,
					row_number() OVER (PARTITION BY f.user_id, f.state ORDER BY f.score DESC) AS place
				FROM feed_entries f JOIN profiles p ON p.id = f.user_id
				WHERE p.locale <> 'en'
			)
			SELECT DISTINCT sh.story_id, sh.locale
			FROM shown sh JOIN stories s ON s.id = sh.story_id
			WHERE (sh.delivered_on = $1::date OR (sh.state = 'candidate' AND sh.place <= $2))
				AND EXISTS (SELECT 1 FROM summaries m WHERE m.story_id = s.id AND m.revision = s.revision)
				AND NOT EXISTS (SELECT 1 FROM translations t WHERE t.kind = 'story' AND t.object_id = s.id
					AND t.revision = s.revision AND t.locale = sh.locale)
			ORDER BY 1, 2`, today.Format(time.DateOnly), shownPerReader)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a TranslateArgs
			if err := rows.Scan(&a.StoryID, &a.Locale); err != nil {
				return err
			}
			out = append(out, a)
		}
		return rows.Err()
	})
	return out, err
}

// SweepArgs enqueues every missing translation.
type SweepArgs struct{}

func (SweepArgs) Kind() string { return "translate.sweep" }

// InsertOpts keeps at most one pending sweep, so every rank can ask for one.
func (SweepArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateScheduled,
	}}}
}

type SweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	Engine *drel.Engine
	Now    func() time.Time
}

func (w *SweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	missing, err := Missing(ctx, w.Engine, w.Now().UTC())
	if err != nil || len(missing) == 0 {
		return err
	}
	params := make([]river.InsertManyParams, len(missing))
	for i, m := range missing {
		params[i] = river.InsertManyParams{Args: m}
	}
	_, err = river.ClientFromContext[pgx.Tx](ctx).InsertMany(ctx, params)
	return err
}

// SweepEveryQuarterHour is the periodic translation sweep.
func SweepEveryQuarterHour() *river.PeriodicJob {
	return river.NewPeriodicJob(river.PeriodicInterval(15*time.Minute),
		func() (river.JobArgs, *river.InsertOpts) { return SweepArgs{}, nil },
		&river.PeriodicJobOpts{RunOnStart: true})
}
