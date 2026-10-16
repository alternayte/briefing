package feed

import (
	"context"
	"errors"
	"strings"

	"github.com/alternayte/drel"
	"github.com/alternayte/gx"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/accounts"
	accountsroute "github.com/alternayte/briefing/internal/accounts/route"
	"github.com/alternayte/briefing/internal/feed/route"
	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/ranking"
	"github.com/alternayte/briefing/internal/sources"
	sourcesroute "github.com/alternayte/briefing/internal/sources/route"
)

const (
	minBudget = 3
	maxBudget = 30
)

var errTopic = errors.New("a topic has 1 to 80 characters, and each list holds at most 20")

// InterestView is one interest as settings and assistants see it.
type InterestView struct {
	ID       int64   `json:"id"`
	Text     string  `json:"text"`
	Negative bool    `json:"negative"`
	Weight   float64 `json:"weight"`

	Remove InterestRemoveButtonProps `json:"-"`
}

func listInterests(ctx context.Context, engine *drel.Engine, userID uuid.UUID) ([]InterestView, error) {
	var out []InterestView
	err := engine.WithTx(ctx, func(ctx context.Context) error {
		rows, err := drel.MustFromContext(ctx).Query(ctx, `SELECT id, text, negative, weight FROM interests WHERE user_id = $1 ORDER BY negative, id`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var v InterestView
			if err := rows.Scan(&v.ID, &v.Text, &v.Negative, &v.Weight); err != nil {
				return err
			}
			out = append(out, v)
		}
		return rows.Err()
	})
	return out, err
}

// Lists the reader's interests: the topics they follow, and with negative
// true, the topics they want left out. Use it before feed_interests_add or
// feed_interests_remove, or to explain why a story was chosen.
var InterestsList = gx.Action(func(c *gx.Ctx, _ route.InterestsList) error {
	ctx := c.R.Context()
	list, err := listInterests(ctx, webFrom(ctx).engine, accounts.CurrentProfile(ctx).ID())
	if err != nil {
		return err
	}
	gx.ToolResult(c, list)
	return nil
}).Tool()

// Adds one interest in the reader's own words, such as "Rust compilers" or
// "exoplanet atmospheres". With negative true the topic is left out of the
// feed instead. The feed re-ranks right after. Returns the reader's interests.
var InterestsAdd = gx.Form(func(c *gx.Ctx, in *route.InterestsAdd) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	text := strings.Join(strings.Fields(in.Text), " ")
	existing, err := listInterests(ctx, w.engine, profile.ID())
	if err != nil {
		return err
	}
	same := 0
	for _, e := range existing {
		if e.Negative == in.Negative {
			same++
		}
	}
	if text == "" || len([]rune(text)) > maxTopicChars || same >= maxTopics {
		return errTopic
	}
	var vector any
	if !in.Negative {
		vs, err := w.embedder.Embed(ctx, "interest_embed", []string{text})
		if err != nil {
			return err
		}
		if vector, err = vs[0].Value(); err != nil {
			return err
		}
	}
	err = w.engine.WithTx(ctx, func(ctx context.Context) error {
		_, err := drel.MustFromContext(ctx).Exec(ctx,
			`INSERT INTO interests (user_id, text, negative, embedding, embed_model) VALUES ($1, $2, $3, $4::vector, $5)`,
			profile.ID(), text, in.Negative, vector, map[bool]string{false: w.embedder.Model(), true: ""}[in.Negative])
		return err
	})
	if err != nil {
		return err
	}
	if err := rerank(ctx, w, profile.ID()); err != nil {
		return err
	}
	list, err := listInterests(ctx, w.engine, profile.ID())
	gx.ToolResult(c, list)
	if err != nil {
		return err
	}
	return c.Redirect(route.Settings{Saved: true})
}, InterestAddForm).Tool()

// Removes one interest by its id from feed_interests_list. The feed re-ranks
// right after. Returns the reader's interests.
var InterestsRemove = gx.Form(func(c *gx.Ctx, in *route.InterestsRemove) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	err := w.engine.WithTx(ctx, func(ctx context.Context) error {
		_, err := drel.MustFromContext(ctx).Exec(ctx, `DELETE FROM interests WHERE id = $1 AND user_id = $2`, in.ID, profile.ID())
		return err
	})
	if err != nil {
		return err
	}
	if err := rerank(ctx, w, profile.ID()); err != nil {
		return err
	}
	list, err := listInterests(ctx, w.engine, profile.ID())
	gx.ToolResult(c, list)
	if err != nil {
		return err
	}
	return c.Redirect(route.Settings{Saved: true})
}, InterestRemoveButton).Tool(gx.Confirm)

func rerank(ctx context.Context, w *Web, userID uuid.UUID) error {
	if w.jobs == nil {
		return nil
	}
	_, err := w.jobs.Insert(ctx, ranking.RankArgs{UserID: userID}, nil)
	return err
}

// SetLocale changes the reader's language.
var SetLocale = gx.Form(func(c *gx.Ctx, in *route.SetLocale) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	l, ok := i18n.Parse(in.Locale)
	if !ok {
		return gx.NotFound()
	}
	if err := w.profiles.SetLocale(ctx, profile.ID(), l); err != nil {
		return err
	}
	return c.Redirect(route.Settings{Saved: true})
}, LocaleForm)

// SetBudget changes how many stories a day the feed shows.
var SetBudget = gx.Form(func(c *gx.Ctx, in *route.SetBudget) error {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	budget := min(maxBudget, max(minBudget, in.Budget))
	if err := w.profiles.SetDailyBudget(ctx, profile.ID(), budget); err != nil {
		return err
	}
	if err := rerank(ctx, w, profile.ID()); err != nil {
		return err
	}
	return c.Redirect(route.Settings{Saved: true})
}, BudgetForm)

// SourceRow is one source in settings, with its follow form.
type SourceRow struct {
	sources.Listed
	Toggle sources.SubscribeButtonProps
}

// TokenRow is one access token in settings.
type TokenRow struct {
	accounts.Token
	Created  string
	LastUsed string
}

// SettingsPage gathers every setting of the reader.
var SettingsPage = gx.Page(func(c *gx.Ctx, in route.Settings) (SettingsViewProps, error) {
	ctx := c.R.Context()
	w, profile := webFrom(ctx), accounts.CurrentProfile(ctx)
	locale := i18n.FromContext(ctx)
	msg := i18n.For(locale)
	p := SettingsViewProps{Msg: msg, MCPEndpoint: w.baseURL + "/mcp", NewToken: accounts.TakeNewToken(c.W, c.R)}
	if in.Saved {
		p.Notice = msg.SettingsSaved
	}

	p.Locale = SetLocale.Props(&route.SetLocale{Locale: string(locale)})
	p.Locale.Current, p.Locale.Label, p.Locale.Save = string(locale), msg.OnboardingLanguage, msg.SettingsSave
	p.Budget = SetBudget.Props(&route.SetBudget{Budget: profile.DailyBudget})
	p.Budget.Current, p.Budget.Label, p.Budget.Hint, p.Budget.Save = profile.DailyBudget, msg.SettingsBudget, msg.SettingsBudgetHint, msg.SettingsSave

	interests, err := listInterests(ctx, w.engine, profile.ID())
	if err != nil {
		return p, err
	}
	for i := range interests {
		interests[i].Remove = InterestsRemove.Props(&route.InterestsRemove{ID: interests[i].ID})
		interests[i].Remove.Label = msg.SettingsRemove
		if interests[i].Negative {
			p.Avoid = append(p.Avoid, interests[i])
		} else {
			p.Interests = append(p.Interests, interests[i])
		}
	}
	p.AddInterest = InterestsAdd.Props(&route.InterestsAdd{})
	p.AddInterest.Label, p.AddInterest.Placeholder = msg.SettingsAdd, msg.SettingsNewInterest
	p.AddAvoid = InterestsAdd.Props(&route.InterestsAdd{Negative: true})
	p.AddAvoid.Label, p.AddAvoid.Placeholder, p.AddAvoid.Negative = msg.SettingsAdd, msg.SettingsNewAvoid, true

	listed, err := sources.ForReader(ctx, w.engine, profile.ID())
	if err != nil {
		return p, err
	}
	for _, l := range listed {
		row := SourceRow{Listed: l, Toggle: sources.Subscribe.Props(&sourcesroute.Subscribe{SourceID: l.ID, On: !l.Subscribed})}
		row.Toggle.On, row.Toggle.Label = !l.Subscribed, map[bool]string{true: msg.SettingsUnsubscribe, false: msg.SettingsSubscribe}[l.Subscribed]
		p.Sources = append(p.Sources, row)
	}
	p.AddFeed = sources.AddRSS.Props(&sourcesroute.AddRSS{})
	p.AddFeed.Label, p.AddFeed.Placeholder = msg.SettingsAddFeed, msg.SettingsFeedURL
	p.FeedNotice = map[string]string{"added": msg.SettingsFeedAdded, "badurl": msg.SettingsFeedBadURL, "failed": msg.SettingsFeedFailed, "notfeed": msg.SettingsNotAFeed}[in.Feed]

	tokens, err := accounts.Tokens(ctx, msg.SettingsTokenRevoke)
	if err != nil {
		return p, err
	}
	for _, t := range tokens {
		row := TokenRow{Token: t, Created: i18n.DateTime(locale, t.CreatedAt)}
		if t.LastUsedAt != nil {
			row.LastUsed = i18n.DateTime(locale, *t.LastUsedAt)
		}
		p.Tokens = append(p.Tokens, row)
	}
	p.CreateToken = accounts.CreateToken.Props(&accountsroute.CreateToken{})
	p.CreateToken.Label, p.CreateToken.Placeholder = msg.SettingsTokenCreate, msg.SettingsTokenName
	return p, nil
}, SettingsView)
