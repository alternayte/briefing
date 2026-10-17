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
	if profile := accounts.CurrentProfile(c.R.Context()); profile != nil {
		so := accounts.SignOut.Props(&route.SignOut{})
		so.Label = msg.SignOut
		p.SignOut = &so
		if profile.Timezone == "" {
			var tz accounts.TimezoneAutoProps
			tz.GxSetForm((&route.SetTimezone{Auto: true}).GxFormValue(nil))
			p.Timezone = &tz
		}
	}
	return p, nil
}, func(p ShellProps, children gx.Node) gx.Node {
	if p.Timezone != nil {
		p.TimezoneScript = gx.El("script", gx.Attrs{{Key: "type", Value: "module", Kind: gx.AttrText}}, gx.Raw(timezoneJS)) //gx:trusted a constant script
	}
	p.Children = children
	return Shell(p)
})

// timezoneJS sends the browser's time zone once, through the hidden Gx form,
// while the profile has none. A module runs after the Gx runtime, so the
// runtime submits the form without a page load.
const timezoneJS = `const f = document.querySelector("form[data-gx-form=setTimezone][hidden]");
const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
if (f && zone) { f.elements[0].value = zone; f.requestSubmit(); }`
