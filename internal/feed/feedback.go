package feed

import (
	"github.com/alternayte/gx"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/feed/route"
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
	if err := rate(ctx, webFrom(ctx).engine, profile.ID(), in.StoryID, in.Kind); err != nil {
		return err
	}
	gx.ToolResult(c, FeedbackResult{StoryID: in.StoryID, Kind: in.Kind})
	if in.From == "story" {
		return c.Redirect(route.Story{ID: in.StoryID})
	}
	return c.Redirect(route.Home{})
}, FeedbackButton).Tool()

// feedbackButtons builds the four feedback forms of a story, marking the
// kinds the reader already gave.
func feedbackButtons(storyID int64, from string, given []string, labels [4]string) []FeedbackButtonProps {
	kinds := [4]string{"useful", "knew", "irrelevant", "misleading"}
	out := make([]FeedbackButtonProps, 4)
	for i, k := range kinds {
		b := Feedback.Props(&route.Feedback{StoryID: storyID, Kind: k, From: from})
		b.Label, b.Kind, b.From = labels[i], k, from
		for _, g := range given {
			b.Chosen = b.Chosen || g == k
		}
		out[i] = b
	}
	return out
}
