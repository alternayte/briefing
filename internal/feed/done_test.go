package feed

import (
	"context"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

// storyIDs returns the story ids of cards, in order.
func storyIDs(cards []Card) []int64 {
	out := []int64{}
	for _, c := range cards {
		out = append(out, c.StoryID)
	}
	return out
}

func entryState(t *testing.T, ctx context.Context, tx *drel.Tx, user uuid.UUID, story int64) string {
	t.Helper()
	var state string
	if err := tx.QueryRow(ctx, `SELECT state::text FROM feed_entries WHERE user_id = $1 AND story_id = $2`, user, story).Scan(&state); err != nil {
		t.Fatal(err)
	}
	return state
}

// "Not relevant" takes a story off Today and lowers the nearest interest;
// undo puts both back, and a second undo changes nothing.
func TestNotRelevantLeavesTodayAndUndoPutsItBack(t *testing.T) {
	e := pgtest.Engine(t)
	now := time.Date(2031, 8, 1, 12, 0, 0, 0, time.UTC)
	tu := platform.DefaultTuning()
	v := view{locale: i18n.EN, loc: time.UTC, now: now}
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		user := pgtest.User(t, ctx, "undo@example.com")
		story := deliveredStory(t, ctx, tx, user, now, "Probe lands", now.Add(-time.Hour))
		_, _ = tx.Exec(ctx, `UPDATE stories SET centroid = '[1,0,0]' WHERE id = $1`, story)
		var interest int64
		if err := tx.QueryRow(ctx, `INSERT INTO interests (user_id, text, weight, embedding) VALUES ($1, 'space', 1, '[1,0,0]') RETURNING id`, user).Scan(&interest); err != nil {
			t.Fatal(err)
		}
		weight := func() float64 {
			var w float64
			_ = tx.QueryRow(ctx, `SELECT weight FROM interests WHERE id = $1`, interest).Scan(&w)
			return w
		}

		if err := rate(ctx, e, user, story, "irrelevant", now, tu.Review); err != nil {
			t.Fatal(err)
		}
		today, _ := todayCards(ctx, tx, user, v)
		done, _ := doneCards(ctx, tx, user, v)
		if len(today) != 0 || len(done) != 1 || done[0].Restore != nil {
			t.Fatalf("after not relevant: today %v, done %v", storyIDs(today), storyIDs(done))
		}
		if w := weight(); w > 0.91 || w < 0.89 {
			t.Errorf("weight after not relevant = %v, want 0.9", w)
		}

		for range 2 {
			if err := unrate(ctx, e, user, story, "irrelevant"); err != nil {
				t.Fatal(err)
			}
		}
		today, _ = todayCards(ctx, tx, user, v)
		if len(today) != 1 || entryState(t, ctx, tx, user, story) != "delivered" {
			t.Errorf("after undo: today %v, state %s", storyIDs(today), entryState(t, ctx, tx, user, story))
		}
		if w := weight(); w > 1.01 || w < 0.99 {
			t.Errorf("weight after undo = %v, want 1", w)
		}
	})
}

// A story marked done comes back with a new revision; one marked "knew it"
// does not. Clear read takes only the read stories.
func TestDoneReturnsOnANewRevisionButKnewDoesNot(t *testing.T) {
	e := pgtest.Engine(t)
	now := time.Date(2031, 8, 1, 12, 0, 0, 0, time.UTC)
	tu := platform.DefaultTuning()
	v := view{locale: i18n.EN, loc: time.UTC, now: now}
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		user := pgtest.User(t, ctx, "done@example.com")
		read := deliveredStory(t, ctx, tx, user, now, "Read one", now.Add(-time.Hour))
		knew := deliveredStory(t, ctx, tx, user, now, "Known one", now.Add(-time.Hour))
		unread := deliveredStory(t, ctx, tx, user, now, "Unread one", now.Add(-time.Hour))
		_, _ = tx.Exec(ctx, `UPDATE feed_entries SET state = 'seen' WHERE story_id = $1`, read)

		if n, err := markDone(ctx, e, user, 0, true, v.day()); err != nil || n != 1 {
			t.Fatalf("clear read changed %d, %v", n, err)
		}
		if err := rate(ctx, e, user, knew, "knew", now, tu.Review); err != nil {
			t.Fatal(err)
		}
		today, _ := todayCards(ctx, tx, user, v)
		if got := storyIDs(today); len(got) != 1 || got[0] != unread {
			t.Fatalf("today after clear read and knew = %v, want [%d]", got, unread)
		}

		_, _ = tx.Exec(ctx, `UPDATE stories SET revision = 2 WHERE id = ANY($1)`, []int64{read, knew})
		today, _ = todayCards(ctx, tx, user, v)
		done, _ := doneCards(ctx, tx, user, v)
		if got := storyIDs(today); len(got) != 2 || !today[0].Updated && !today[1].Updated {
			t.Errorf("today after new revisions = %v, want the read story back marked updated", got)
		}
		if got := storyIDs(done); len(got) != 1 || got[0] != knew {
			t.Errorf("done after new revisions = %v, want [%d]", got, knew)
		}
	})
}

// "Not relevant" on a "More" card sets it done without a delivery date, so
// it leaves More and the ranker never delivers it.
func TestNotRelevantOnMoreIsNeverDelivered(t *testing.T) {
	e := pgtest.Engine(t)
	now := time.Date(2031, 8, 1, 12, 0, 0, 0, time.UTC)
	tu := platform.DefaultTuning()
	v := view{locale: i18n.EN, loc: time.UTC, now: now}
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		user := pgtest.User(t, ctx, "more@example.com")
		story := deliveredStory(t, ctx, tx, user, now, "Waiting", now.Add(-time.Hour))
		_, _ = tx.Exec(ctx, `UPDATE feed_entries SET state = 'candidate', delivered_on = NULL, delivered_at = NULL WHERE story_id = $1`, story)
		if err := rate(ctx, e, user, story, "irrelevant", now, tu.Review); err != nil {
			t.Fatal(err)
		}
		more, _ := moreCards(ctx, tx, user, now.Add(-72*time.Hour), 10, v)
		if len(more) != 0 || entryState(t, ctx, tx, user, story) != "done" {
			t.Errorf("more %v, state %s; want none and done", storyIDs(more), entryState(t, ctx, tx, user, story))
		}
		if err := unrate(ctx, e, user, story, "irrelevant"); err != nil {
			t.Fatal(err)
		}
		if s := entryState(t, ctx, tx, user, story); s != "candidate" {
			t.Errorf("state after undo = %s, want candidate", s)
		}
	})
}

// Today is the reader's local day: at 23:45 UTC a reader in Paris is on the
// next date already.
func TestTodayFollowsTheReadersTimeZone(t *testing.T) {
	e := pgtest.Engine(t)
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2031, 8, 1, 23, 45, 0, 0, time.UTC)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		user := pgtest.User(t, ctx, "paris@example.com")
		local := now.In(paris)
		story := deliveredStory(t, ctx, tx, user, local, "Late news", now.Add(-time.Minute))
		utcToday, _ := todayCards(ctx, tx, user, view{locale: i18n.EN, loc: time.UTC, now: now})
		parisToday, _ := todayCards(ctx, tx, user, view{locale: i18n.EN, loc: paris, now: now})
		if len(utcToday) != 0 || len(parisToday) != 1 || parisToday[0].StoryID != story {
			t.Errorf("UTC today %v, Paris today %v; want the story on the Paris day only", storyIDs(utcToday), storyIDs(parisToday))
		}
	})
}
