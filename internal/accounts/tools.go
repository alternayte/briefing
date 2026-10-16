package accounts

import (
	"github.com/alternayte/gx"

	"github.com/alternayte/briefing/internal/accounts/route"
)

// Identity is the signed-in user as an assistant sees it.
type Identity struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Locale      string `json:"locale"`
	DailyBudget int    `json:"daily_budget"`
}

// Returns the Briefing user that this connection acts for: their id, display
// name, language (en or fr) and daily story budget. Call it first to check the
// connection works and to learn which language to answer in.
var me = gx.Action(func(c *gx.Ctx, in route.Me) error {
	p := CurrentProfile(c.R.Context())
	if p == nil {
		return gx.Forbidden()
	}
	gx.ToolResult(c, Identity{
		UserID:      p.ID().String(),
		DisplayName: p.DisplayName,
		Locale:      string(p.UserLocale()),
		DailyBudget: p.DailyBudget,
	})
	return nil
}).Tool()
