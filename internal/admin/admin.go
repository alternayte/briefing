// Package admin is the operations page: sources, jobs and model spend.
package admin

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/gx"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/admin/route"
	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/sources"
)

// Web holds what the admin pages read at request time.
type Web struct {
	engine *drel.Engine
	capUSD float64
	now    func() time.Time
}

func NewWeb(engine *drel.Engine, cfg platform.Config, now func() time.Time) *Web {
	return &Web{engine: engine, capUSD: cfg.DailySpendUSD, now: now}
}

type webKey struct{}

// Middleware puts Web in the request context and answers 404 to anyone who
// is not an admin, so the page does not reveal itself.
func (w *Web) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if !accounts.IsAdmin(r.Context()) {
			http.NotFound(rw, r)
			return
		}
		next.ServeHTTP(rw, r.WithContext(context.WithValue(r.Context(), webKey{}, w)))
	})
}

func webFrom(ctx context.Context) *Web { return ctx.Value(webKey{}).(*Web) }

// SourceRow is one source as the page shows it.
type SourceRow struct {
	Toggle      ToggleButtonProps
	Title       string
	Kind        string
	Enabled     bool
	LastSuccess string
	LastError   string
}

type JobCount struct {
	Kind  string
	State string
	Count string
}

type Failure struct {
	Kind  string
	At    string
	Error string
}

// Flag is one summary a reader flagged as misleading.
type Flag struct {
	StoryURL string
	Headline string
	Reader   string
	At       string
}

type SpendRow struct {
	Stage string
	Model string
	Calls string
	Cost  string
}

// Page is the operations page.
var Page = gx.Page(func(c *gx.Ctx, _ route.Admin) (DashboardProps, error) {
	ctx := c.R.Context()
	w := webFrom(ctx)
	locale := i18n.FromContext(ctx)
	msg := i18n.For(locale)
	p := DashboardProps{Msg: msg}

	srcs, err := sources.List(ctx, w.engine)
	if err != nil {
		return p, err
	}
	for _, s := range srcs {
		row := SourceRow{Title: s.Title, Kind: string(s.Kind), Enabled: s.Enabled, LastSuccess: msg.AdminNever, LastError: s.LastError}
		if s.LastOKAt != nil {
			row.LastSuccess = i18n.DateTime(locale, *s.LastOKAt)
		}
		row.Toggle = ToggleSource.Props(&route.ToggleSource{ID: s.ID()})
		row.Toggle.Label = msg.AdminDisable
		if !s.Enabled {
			row.Toggle.Label = msg.AdminEnable
		}
		p.Sources = append(p.Sources, row)
	}

	now := w.now().UTC()
	err = w.engine.WithTx(ctx, func(ctx context.Context) error {
		tx := drel.MustFromContext(ctx)
		rows, err := tx.Query(ctx, `SELECT kind, state::text, count(*) FROM river_job GROUP BY 1, 2 ORDER BY 1, 2`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var j JobCount
			var n int64
			if err := rows.Scan(&j.Kind, &j.State, &n); err != nil {
				rows.Close()
				return err
			}
			j.Count = i18n.Int(locale, n)
			p.Jobs = append(p.Jobs, j)
		}
		rows.Close()
		rows, err = tx.Query(ctx, `
			SELECT kind, COALESCE(attempted_at, created_at), COALESCE(errors[array_length(errors, 1)]->>'error', '')
			FROM river_job WHERE errors IS NOT NULL ORDER BY id DESC LIMIT 10`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var f Failure
			var at time.Time
			if err := rows.Scan(&f.Kind, &at, &f.Error); err != nil {
				rows.Close()
				return err
			}
			f.At = i18n.DateTime(locale, at)
			p.Failures = append(p.Failures, f)
		}
		rows.Close()
		rows, err = tx.Query(ctx, `
			SELECT e.story_id, COALESCE((SELECT headline FROM summaries WHERE story_id = e.story_id ORDER BY revision DESC LIMIT 1), ''),
				COALESCE(NULLIF(p.display_name, ''), u.email), e.created_at
			FROM learning_events e JOIN auth_users u ON u.id = e.user_id LEFT JOIN profiles p ON p.id = e.user_id
			WHERE e.kind = 'misleading' ORDER BY e.id DESC LIMIT 50`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var f Flag
			var storyID int64
			var at time.Time
			if err := rows.Scan(&storyID, &f.Headline, &f.Reader, &at); err != nil {
				rows.Close()
				return err
			}
			f.StoryURL, f.At = "/s/"+strconv.FormatInt(storyID, 10), i18n.DateTime(locale, at)
			p.Flags = append(p.Flags, f)
		}
		rows.Close()
		rows, err = tx.Query(ctx, `SELECT stage, model, calls, cost_usd::float8 FROM llm_usage WHERE day = $1::date ORDER BY cost_usd DESC`, now.Format(time.DateOnly))
		if err != nil {
			return err
		}
		defer rows.Close()
		var total float64
		for rows.Next() {
			var r SpendRow
			var calls int64
			var cost float64
			if err := rows.Scan(&r.Stage, &r.Model, &calls, &cost); err != nil {
				return err
			}
			total += cost
			r.Calls, r.Cost = i18n.Int(locale, calls), i18n.USD(locale, cost)
			p.Spend = append(p.Spend, r)
		}
		p.SpendTotal, p.SpendCap = i18n.USD(locale, total), i18n.USD(locale, w.capUSD)
		p.CapReached = total >= w.capUSD
		return rows.Err()
	})
	return p, err
}, Dashboard)

// ToggleSource turns a source on or off and returns to the page.
var ToggleSource = gx.Form(func(c *gx.Ctx, in *route.ToggleSource) error {
	ctx := c.R.Context()
	w := webFrom(ctx)
	src, err := sources.Find(ctx, w.engine, in.ID)
	if err != nil {
		return gx.NotFound()
	}
	if err := sources.SetEnabled(ctx, w.engine, src.ID(), !src.Enabled); err != nil {
		return err
	}
	return c.Redirect(route.Admin{})
}, ToggleButton)

// Routes lists every page and form of the slice.
var Routes = gx.Collect(Page, ToggleSource)
