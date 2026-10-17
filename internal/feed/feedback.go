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
	"github.com/alternayte/briefing/internal/i18n"
)

// FeedbackResult is what an assistant gets back from feed_feedback.
type FeedbackResult struct {
	StoryID int64  `json:"story_id"`
	Kind    string `json:"kind"`
}

// Records the reader's feedback on a story: "useful" (worth reading; it also
// schedules the story's review questions), "knew" (the reader already knew
// it, so similar stories rank lower), "irrelevant" (not for this reader) or
// "misleading" (the entry misstates its sources). Use it when the reader says
// so about a story you showed them. Returns the story id and the recorded kind.
var Feedback = gx.Form(func(c *gx.Ctx, in *route.Feedback) error {
	ctx := c.R.Context()
	profile := accounts.CurrentProfile(ctx)
	w := webFrom(ctx)
	if err := rate(ctx, w.engine, profile.ID(), in.StoryID, in.Kind, w.now().UTC(), w.tuning.Review); err != nil {
		return err
	}
	gx.ToolResult(c, FeedbackResult{StoryID: in.StoryID, Kind: in.Kind})
	if !fromPage(c.R) {
		if in.From == "story" {
			return c.Redirect(route.Read{ID: in.StoryID})
		}
		return c.Redirect(route.Home{})
	}
	// In the page the answer patches the story in place, so the reader keeps
	// their place in the feed.
	l := i18n.FromContext(ctx)
	msg := i18n.For(l)
	return w.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		if in.From == "story" {
			given, err := givenFeedback(ctx, tx, profile.ID(), in.StoryID)
			if err != nil {
				return err
			}
			return c.Patch(FeedbackGroup(FeedbackGroupProps{StoryID: in.StoryID, Buttons: feedbackButtons(in.StoryID, "story", given, feedbackLabels(msg)), Msg: msg}))
		}
		cards, err := queryCards(ctx, tx, l, cardSelect+` WHERE s.id = $2`, profile.ID(), in.StoryID)
		if err != nil || len(cards) == 0 {
			return err
		}
		return c.Patch(StoryCard(StoryCardProps{Card: cards[0], Msg: msg}))
	})
}, FeedbackButton).Tool()

// entryID is the element id of a story card, which a feedback answer patches.
func entryID(storyID int64) string { return "entry-" + strconv.FormatInt(storyID, 10) }

// feedbackID is the element id of the feedback buttons of a story.
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
