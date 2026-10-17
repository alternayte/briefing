package feed

import (
	"context"
	"net/http"
	"strconv"

	"github.com/alternayte/drel"
	"github.com/alternayte/gx"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/feed/route"
)

// FeedbackResult is what an assistant gets back from feed_feedback.
type FeedbackResult struct {
	StoryID int64  `json:"story_id"`
	Kind    string `json:"kind"`
	Undo    bool   `json:"undo,omitempty"`
}

// Records the reader's feedback on a story: "useful" (worth reading; it also
// schedules the story's review questions), "knew" (the reader already knew
// it, so similar stories rank lower; the story leaves Today), "irrelevant"
// (not for this reader; the story leaves Today) or "misleading" (the entry
// misstates its sources; its summary is hidden for the reader). With undo
// true it removes that feedback and reverses its effect; feedback that is
// not there changes nothing, so a retry is safe. Use it when the reader says
// so about a story you showed them. Returns the story id and the kind.
var Feedback = gx.Form(func(c *gx.Ctx, in *route.Feedback) error {
	ctx := c.R.Context()
	profile := accounts.CurrentProfile(ctx)
	w := webFrom(ctx)
	var err error
	if in.Undo {
		err = unrate(ctx, w.engine, profile.ID(), in.StoryID, in.Kind)
	} else {
		err = rate(ctx, w.engine, profile.ID(), in.StoryID, in.Kind, w.now().UTC(), w.tuning.Review)
	}
	if err != nil {
		return err
	}
	gx.ToolResult(c, FeedbackResult{StoryID: in.StoryID, Kind: in.Kind, Undo: in.Undo})
	if !fromPage(c.R) {
		if in.From == "story" {
			return c.Redirect(route.Read{ID: in.StoryID})
		}
		return c.Redirect(route.Home{})
	}
	// In the page the answer patches the page in place, so the reader keeps
	// their place.
	if in.From == "story" {
		p, err := storyView(ctx, route.Read{ID: in.StoryID}, false)
		if err != nil {
			return err
		}
		return c.Patch(FeedbackGroup(FeedbackGroupProps{StoryID: in.StoryID, Buttons: p.Feedback, Msg: p.Msg}), StoryViewSummary(p, p.Flagged))
	}
	return patchHome(c)
}, FeedbackButton).Tool()

// DoneResult is what an assistant gets back from feed_done and
// feed_restore: how many stories changed.
type DoneResult struct {
	Changed int64 `json:"changed"`
}

// Takes stories off Today and lists them under "Done today": one story by
// its story_id, or with all_seen true every story of today the reader opened
// or rated. A story marked done comes back to Today when its summary is
// updated. Done stories still count toward the daily budget. Use it when the
// reader is done with a story or wants to clear what they read. Returns how
// many stories changed.
var DoneAction = gx.Form(func(c *gx.Ctx, in *route.Done) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	if in.StoryID == 0 && !in.AllSeen {
		return gx.FieldError(&in.StoryID, "required")
	}
	n, err := markDone(ctx, w.engine, profile.ID(), in.StoryID, in.AllSeen, accounts.LocalDay(w.now(), profile.Location()))
	if err != nil {
		return err
	}
	gx.ToolResult(c, DoneResult{Changed: n})
	if !fromPage(c.R) {
		return c.Redirect(route.Home{})
	}
	return patchHome(c)
}, DoneButton).Tool()

// Brings a story the reader marked done back to Today, by its story_id. A
// story the reader marked "knew" or "irrelevant" comes back only when that
// feedback is undone with feed_feedback. Returns how many stories changed.
var RestoreAction = gx.Form(func(c *gx.Ctx, in *route.Restore) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	n, err := restore(ctx, w.engine, profile.ID(), in.StoryID)
	if err != nil {
		return err
	}
	gx.ToolResult(c, DoneResult{Changed: n})
	if !fromPage(c.R) {
		return c.Redirect(route.Home{})
	}
	return patchHome(c)
}, RestoreButton).Tool()

// markDone sets delivered entries done: one story, or with allSeen every
// read entry of day. It keeps the revision the reader saw, so a newer one
// brings the story back.
func markDone(ctx context.Context, engine *drel.Engine, userID uuid.UUID, storyID int64, allSeen bool, day string) (int64, error) {
	var n int64
	err := engine.WithTx(ctx, func(ctx context.Context) (err error) {
		tx := drel.MustFromContext(ctx)
		if allSeen {
			n, err = tx.Exec(ctx, `
				UPDATE feed_entries f SET state = 'done', shown_revision = s.revision, updated_at = now()
				FROM stories s WHERE s.id = f.story_id AND f.user_id = $1 AND f.delivered_on = $2::date AND f.state = 'seen'`, userID, day)
			return err
		}
		n, err = tx.Exec(ctx, `
			UPDATE feed_entries f SET state = 'done', shown_revision = s.revision, updated_at = now()
			FROM stories s WHERE s.id = f.story_id AND f.user_id = $1 AND f.story_id = $2 AND f.delivered_on IS NOT NULL`, userID, storyID)
		return err
	})
	return n, err
}

// restore returns a done entry to Today as seen, unless the reader marked
// the story knew or irrelevant.
func restore(ctx context.Context, engine *drel.Engine, userID uuid.UUID, storyID int64) (int64, error) {
	var n int64
	err := engine.WithTx(ctx, func(ctx context.Context) (err error) {
		n, err = drel.MustFromContext(ctx).Exec(ctx, `
			UPDATE feed_entries SET state = 'seen', updated_at = now()
			WHERE user_id = $1 AND story_id = $2 AND state = 'done' AND delivered_on IS NOT NULL AND NOT EXISTS (
				SELECT 1 FROM learning_events WHERE user_id = $1 AND story_id = $2 AND kind IN ('knew', 'irrelevant'))`, userID, storyID)
		return err
	})
	return n, err
}

// doneButton builds the Done form of one story, or with allSeen the Clear
// read form.
func doneButton(storyID int64, allSeen bool, label string) DoneButtonProps {
	var b DoneButtonProps
	b.GxSetForm((&route.Done{StoryID: storyID, AllSeen: allSeen}).GxFormValue(nil))
	b.Label, b.StoryID, b.AllSeen = label, storyID, allSeen
	return b
}

// restoreButton builds the Restore form of one story.
func restoreButton(storyID int64, label string) RestoreButtonProps {
	var b RestoreButtonProps
	b.GxSetForm((&route.Restore{StoryID: storyID}).GxFormValue(nil))
	b.Label, b.StoryID = label, storyID
	return b
}

// entryID is the element id of a story card.
func entryID(storyID int64) string { return "entry-" + strconv.FormatInt(storyID, 10) }

// feedbackID is the element id of the feedback buttons of a story, which a
// feedback answer on the story page patches.
func feedbackID(storyID int64) string { return "feedback-" + strconv.FormatInt(storyID, 10) }

// fromPage reports whether the request comes from the Gx runtime in an open
// page, which can take a patch; a plain form post or a tool call cannot.
func fromPage(r *http.Request) bool { return r.Header.Get("Datastar-Request") != "" }

// givenFeedback returns the feedback kinds the reader gave on a story.
func givenFeedback(ctx context.Context, tx *drel.Tx, userID uuid.UUID, storyID int64) ([]string, error) {
	var given []string
	err := tx.QueryRow(ctx, `SELECT COALESCE(array_agg(DISTINCT kind::text), '{}') FROM learning_events
		WHERE user_id = $1 AND story_id = $2 AND kind IN ('useful', 'knew', 'irrelevant', 'misleading')`, userID, storyID).Scan(&given)
	return given, err
}

// feedbackButtons builds the four feedback forms of a story, marking the
// kinds the reader already gave.
func feedbackButtons(storyID int64, from string, given []string, labels [4]string) []FeedbackButtonProps {
	kinds := [4]string{"useful", "knew", "irrelevant", "misleading"}
	out := make([]FeedbackButtonProps, 4)
	for i, k := range kinds {
		// The form value comes straight from the route input; going through
		// Feedback.Props would make Feedback depend on itself.
		var b FeedbackButtonProps
		b.GxSetForm((&route.Feedback{StoryID: storyID, Kind: k, From: from}).GxFormValue(nil))
		b.Label, b.Kind, b.From = labels[i], k, from
		for _, g := range given {
			b.Chosen = b.Chosen || g == k
		}
		out[i] = b
	}
	return out
}
