package feed

import (
	"errors"
	"fmt"

	"github.com/alternayte/gx"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/feed/route"
	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/translate"
)

// storySource links to one source of the story page.
type storySource struct {
	storyID int64
	notice  string
}

func (s storySource) URL() string {
	u := fmt.Sprintf("/s/%d", s.storyID)
	if s.notice != "" {
		u += "?article=" + s.notice
	}
	return u + "#sources"
}

// TranslateArticle translates one source's text into the reader's language.
var TranslateArticle = gx.Form(func(c *gx.Ctx, in *route.TranslateArticle) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	_, err := w.translator.TranslateArticle(ctx, profile.ID(), in.ItemID, i18n.FromContext(ctx))
	switch {
	case errors.Is(err, translate.ErrCapReached):
		return c.Redirect(storySource{in.StoryID, "cap"})
	case errors.Is(err, translate.ErrArticleLimit):
		return c.Redirect(storySource{in.StoryID, "limit"})
	case errors.Is(err, translate.ErrNothing):
		return gx.NotFound()
	case err != nil:
		return err
	}
	return c.Redirect(storySource{storyID: in.StoryID})
}, TranslateArticleButton)
