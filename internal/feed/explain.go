package feed

import (
	"errors"

	"github.com/alternayte/gx"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/feed/route"
	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/write"
)

// ExplainResult is what an assistant gets back from feed_explain.
type ExplainResult struct {
	StoryID     int64  `json:"story_id"`
	Language    string `json:"language"`
	Explanation string `json:"explanation,omitempty"`
	// Notice says why there is no explanation, when there is none.
	Notice string `json:"notice,omitempty"`
}

// Explains a story at the reader's level, in their language: it builds on the
// concepts they have mastered and defines the ones they have not met. Use it
// when the reader asks what a story means or why it matters. A story has one
// explanation per revision; asking again returns the same text. Returns the
// explanation as plain text, or an error when today's limit or spending cap
// is reached.
var Explain = gx.Form(func(c *gx.Ctx, in *route.Explain) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	locale := i18n.FromContext(ctx)
	text, err := w.explainer.Explain(ctx, profile.ID(), in.StoryID, locale)
	switch {
	case errors.Is(err, write.ErrCapReached), errors.Is(err, write.ErrExplainLimit):
		code := "cap"
		if errors.Is(err, write.ErrExplainLimit) {
			code = "limit"
		}
		gx.ToolResult(c, ExplainResult{StoryID: in.StoryID, Language: string(locale), Notice: err.Error()})
		return c.Redirect(route.Story{ID: in.StoryID, Explain: code})
	case errors.Is(err, write.ErrNoEntry):
		return gx.NotFound()
	case err != nil:
		return err
	}
	gx.ToolResult(c, ExplainResult{StoryID: in.StoryID, Language: string(locale), Explanation: text})
	return c.Redirect(route.Story{ID: in.StoryID})
}, ExplainButton).Tool()
