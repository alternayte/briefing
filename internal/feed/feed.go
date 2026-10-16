package feed

import (
	"context"

	"github.com/alternayte/drel"
	"github.com/alternayte/gx"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/feed/route"
	"github.com/alternayte/briefing/internal/i18n"
)

const moreLimit = 10

// HomePage is the feed. A reader who has not finished onboarding goes there
// first.
var HomePage = gx.Page(func(c *gx.Ctx, _ route.Home) (HomeProps, error) {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	if profile.OnboardedAt == nil {
		return HomeProps{}, gx.Redirect(route.Onboarding{})
	}
	now := w.now().UTC()
	p := HomeProps{Msg: i18n.For(i18n.FromContext(ctx)), LoadedAt: now.Unix()}
	err := w.engine.WithTx(ctx, func(ctx context.Context) (err error) {
		tx := drel.MustFromContext(ctx)
		if p.Today, err = todayCards(ctx, tx, profile.ID(), now, i18n.FromContext(ctx)); err != nil {
			return err
		}
		p.CaughtUp = len(p.Today) > 0
		for _, c := range p.Today {
			p.CaughtUp = p.CaughtUp && c.Seen && !c.Updated
		}
		p.More, err = moreCards(ctx, tx, profile.ID(), now.Add(-w.tuning.Cluster.OpenWindow), moreLimit, i18n.FromContext(ctx))
		return err
	})
	return p, err
}, Home)

// FreshAction tells an open feed that stories arrived after it loaded. The
// page calls it every minute and shows a banner when the count is not zero.
var FreshAction = gx.Action(func(c *gx.Ctx, in route.Fresh) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	var n int
	err := w.engine.WithTx(ctx, func(ctx context.Context) error {
		return drel.MustFromContext(ctx).QueryRow(ctx, `
			SELECT count(*) FROM feed_entries
			WHERE user_id = $1 AND delivered_on = $2::date AND updated_at > to_timestamp($3)`,
			profile.ID(), w.now().UTC().Format("2006-01-02"), in.Since).Scan(&n)
	})
	if err != nil || n == 0 {
		return err
	}
	msg := i18n.For(i18n.FromContext(ctx))
	return c.Patch(HomeFresh(msg.FeedNewStories, msg.FeedShow))
})

// Routes lists every page, action and form of the slice.
var Routes = gx.Collect(HomePage, FreshAction, StoryPage, Feedback, Explain, TranslateArticle, SettingsPage, SetLocale, SetBudget, InterestsList, InterestsAdd, InterestsRemove, OnboardingPage, SaveInterests, CalibratePage, Rate, Finish)
