package accounts

import (
	"context"
	"net/http"
	"strings"
	"time"

	authall "github.com/alternayte/auth-all"
	"github.com/alternayte/auth-all/plugins/apikeys"
	"github.com/alternayte/gx"

	"github.com/alternayte/briefing/internal/accounts/route"
)

// newTokenCookie carries a freshly created token to the settings page once.
const newTokenCookie = "briefing_new_token"

// maxTokenName bounds the label of a token, which auth-all limits to 100.
const maxTokenName = 100

// settingsURL is the settings page; its route type belongs to the feed slice.
type settingsURL struct{}

func (settingsURL) URL() string { return "/settings" }

// Token is one access token as the settings page lists it.
type Token struct {
	ID         string
	Name       string
	Start      string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	Revoke     RevokeTokenButtonProps
}

// Tokens lists the signed-in user's live tokens.
func Tokens(ctx context.Context, revokeLabel string) ([]Token, error) {
	w := webFrom(ctx)
	u := authall.UserFrom(ctx)
	keys, err := w.auth.Keys.List(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	var out []Token
	for _, k := range keys {
		if k.RevokedAt != nil {
			continue
		}
		t := Token{ID: k.ID, Name: k.Name, Start: k.Start, CreatedAt: k.CreatedAt, LastUsedAt: k.LastUsedAt}
		t.Revoke = RevokeToken.Props(&route.RevokeToken{ID: k.ID})
		t.Revoke.Label = revokeLabel
		out = append(out, t)
	}
	return out, nil
}

// TakeNewToken returns the token created just before, once, and clears it.
func TakeNewToken(w http.ResponseWriter, r *http.Request) string {
	c, err := r.Cookie(newTokenCookie)
	if err != nil || !strings.HasPrefix(c.Value, AccessTokenPrefix) {
		return ""
	}
	http.SetCookie(w, &http.Cookie{Name: newTokenCookie, Path: "/settings", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	return c.Value
}

// CreateToken makes a token and shows it once on the settings page.
var CreateToken = gx.Form(func(c *gx.Ctx, in *route.CreateToken) error {
	ctx := c.R.Context()
	w, u := webFrom(ctx), authall.UserFrom(ctx)
	if u == nil {
		return gx.Forbidden()
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "MCP"
	}
	if r := []rune(name); len(r) > maxTokenName {
		name = string(r[:maxTokenName])
	}
	_, plain, err := w.auth.Keys.Create(ctx, u, apikeys.CreateInput{UserID: u.ID, Name: name})
	if err != nil {
		return err
	}
	http.SetCookie(c.W, &http.Cookie{
		Name: newTokenCookie, Value: plain, Path: "/settings", MaxAge: 120,
		HttpOnly: true, Secure: w.cfg.BaseURL.Scheme == "https", SameSite: http.SameSiteLaxMode,
	})
	return c.Redirect(settingsURL{})
}, CreateTokenForm)

// RevokeToken ends a token of the signed-in user.
var RevokeToken = gx.Form(func(c *gx.Ctx, in *route.RevokeToken) error {
	ctx := c.R.Context()
	w, u := webFrom(ctx), authall.UserFrom(ctx)
	if u == nil {
		return gx.Forbidden()
	}
	if err := w.auth.Keys.Revoke(ctx, in.ID, u, false); err != nil {
		return err
	}
	return c.Redirect(settingsURL{})
}, RevokeTokenButton)
