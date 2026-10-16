package feed

import (
	"context"

	"github.com/alternayte/drel"
	"github.com/alternayte/gx"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/feed/route"
	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/ranking"
)

const (
	errNeedInterest = "need_interest"
	errTooMany      = "too_many"
)

// OnboardingPage asks for the language and the interests.
var OnboardingPage = gx.Page(func(c *gx.Ctx, in route.Onboarding) (OnboardingProps, error) {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	msg := i18n.For(i18n.FromContext(ctx))
	p := SaveInterests.Props(&route.SaveInterests{Locale: string(profile.UserLocale())})
	p.Msg, p.Locale = msg, string(profile.UserLocale())
	err := w.engine.WithTx(ctx, func(ctx context.Context) (err error) {
		p.Interests, p.Avoid, err = interestsOf(ctx, drel.MustFromContext(ctx), profile.ID())
		return err
	})
	switch in.Error {
	case errNeedInterest:
		p.Error = msg.OnboardingNeedInterest
	case errTooMany:
		p.Error = msg.OnboardingTooMany
	}
	return p, err
}, Onboarding)

// SaveInterests stores the language and the interests, then moves on to
// calibration.
var SaveInterests = gx.Form(func(c *gx.Ctx, in *route.SaveInterests) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	positive, negative := parseTopics(in.Interests), parseTopics(in.Avoid)
	switch {
	case len(positive) == 0:
		return c.Redirect(route.Onboarding{Error: errNeedInterest})
	case !topicsValid(positive) || !topicsValid(negative):
		return c.Redirect(route.Onboarding{Error: errTooMany})
	}
	if l, ok := i18n.Parse(in.Locale); ok && l != profile.UserLocale() {
		if err := w.profiles.SetLocale(ctx, profile.ID(), l); err != nil {
			return err
		}
	}
	if err := replaceInterests(ctx, w.engine, w.embedder, profile.ID(), positive, negative); err != nil {
		return err
	}
	return c.Redirect(route.Calibrate{})
}, Onboarding)

// CalibratePage lists recent stories to rate.
var CalibratePage = gx.Page(func(c *gx.Ctx, _ route.Calibrate) (CalibrateViewProps, error) {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	p := CalibrateViewProps{Msg: i18n.For(i18n.FromContext(ctx)), Finish: Finish.Props(&route.Finish{})}
	p.Finish.Label = p.Msg.CalibrateFinish
	err := w.engine.WithTx(ctx, func(ctx context.Context) error {
		set, err := calibrationSet(ctx, drel.MustFromContext(ctx), profile.ID(), w.now().Add(-w.tuning.Cluster.OpenWindow))
		for _, s := range set {
			row := CalibrationRow{Headline: s.Headline, Summary: s.Summary, Rated: s.Rated}
			for _, r := range []struct{ value, label string }{
				{"useful", p.Msg.CalibrateUseful}, {"knew", p.Msg.CalibrateKnew}, {"irrelevant", p.Msg.CalibrateNotForMe},
			} {
				b := Rate.Props(&route.Rate{StoryID: s.ID, Rating: r.value})
				b.Label, b.Value, b.Chosen = r.label, r.value, s.Rated == r.value
				row.Buttons = append(row.Buttons, b)
			}
			p.Stories = append(p.Stories, row)
		}
		return err
	})
	return p, err
}, CalibrateView)

// Rate records one calibration rating.
var Rate = gx.Form(func(c *gx.Ctx, in *route.Rate) error {
	ctx := c.R.Context()
	if err := rate(ctx, webFrom(ctx).engine, accounts.CurrentProfile(ctx).ID(), in.StoryID, in.Rating); err != nil {
		return err
	}
	return c.Redirect(route.Calibrate{})
}, RateButton)

// Finish ends onboarding and ranks the first feed at once.
var Finish = gx.Form(func(c *gx.Ctx, _ *route.Finish) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	if err := w.profiles.MarkOnboarded(ctx, profile.ID(), w.now().UTC()); err != nil {
		return err
	}
	if _, err := w.jobs.Insert(ctx, ranking.RankArgs{UserID: profile.ID()}, nil); err != nil {
		return err
	}
	return c.Redirect(route.Home{})
}, FinishButton)
