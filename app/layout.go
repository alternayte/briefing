// Package app holds the layout and the theme of the app.
package app

import (
	"github.com/alternayte/gx"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/accounts/route"
	"github.com/alternayte/briefing/internal/i18n"
)

// Layout wraps every page in the shell, in the locale of the request.
var Layout = gx.Layout(func(c *gx.Ctx) (ShellProps, error) {
	locale := i18n.FromContext(c.R.Context())
	msg := i18n.For(locale)
	p := ShellProps{Msg: msg, Lang: string(locale)}
	if accounts.CurrentProfile(c.R.Context()) != nil {
		so := accounts.SignOut.Props(&route.SignOut{})
		so.Label = msg.SignOut
		p.SignOut = &so
	}
	return p, nil
}, func(p ShellProps, children gx.Node) gx.Node {
	p.Children = children
	return Shell(p)
})
