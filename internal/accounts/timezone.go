package accounts

import (
	"github.com/alternayte/gx"

	"github.com/alternayte/briefing/internal/accounts/route"
	"github.com/alternayte/briefing/internal/i18n"
)

// TimezoneResult is what an assistant gets back from accounts_set_timezone.
type TimezoneResult struct {
	Timezone string `json:"timezone"`
	Applied  bool   `json:"applied"`
}

// settingsSavedURL is the settings page with its saved notice.
type settingsSavedURL struct{}

func (settingsSavedURL) URL() string { return "/settings?saved=true" }

// SetTimezone sets the reader's time zone, an IANA name such as
// Europe/Paris. The reader's day starts at midnight in it: today's briefing,
// its daily budget and the daily review cap. With auto, the zone applies only
// while the reader has none; the page sends it from the browser.
var SetTimezone = gx.Form(func(c *gx.Ctx, in *route.SetTimezone) error {
	ctx := c.R.Context()
	p := CurrentProfile(ctx)
	if p == nil {
		return gx.Forbidden()
	}
	applied := !in.Auto || p.Timezone == ""
	if applied {
		if err := webFrom(ctx).profiles.SetTimezone(ctx, p.ID(), in.Timezone); err != nil {
			if in.Auto {
				return nil
			}
			return c.Toast(i18n.For(i18n.FromContext(ctx)).SettingsInvalid, gx.ToastError)
		}
	}
	gx.ToolResult(c, TimezoneResult{Timezone: in.Timezone, Applied: applied})
	if in.Auto {
		return nil
	}
	return c.Redirect(settingsSavedURL{})
}, TimezoneForm).Tool()
