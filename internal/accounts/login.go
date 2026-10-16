package accounts

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"github.com/alternayte/auth-all/apierr"
	"github.com/alternayte/gx"

	"github.com/alternayte/briefing/internal/accounts/route"
	"github.com/alternayte/briefing/internal/i18n"
)

const codeInvalidEmail = "INVALID_EMAIL"

// LoginPage offers GitHub and, when email is configured, a magic link.
var LoginPage = gx.Page(func(c *gx.Ctx, in route.Login) (LoginProps, error) {
	if CurrentProfile(c.R.Context()) != nil {
		return LoginProps{}, c.Redirect(homeURL{})
	}
	msg := i18n.For(i18n.FromContext(c.R.Context()))
	p := SendLink.Props(&route.SendLink{})
	p.Msg = msg
	p.MagicLink = webFrom(c.R.Context()).cfg.MagicLinkEnabled()
	switch {
	case in.Error != "":
		p.Notice, p.NoticeIsError = loginError(msg, in.Error), true
	case in.Sent:
		p.Notice = msg.LoginLinkSent
	}
	return p, nil
}, Login)

// VerifyPage is where a magic link lands. It only shows a button, so a mail
// scanner that opens the link does not use up the token.
var VerifyPage = gx.Page(func(c *gx.Ctx, in route.LoginVerify) (VerifyProps, error) {
	if in.Token == "" {
		return VerifyProps{}, c.Redirect(route.Login{})
	}
	return VerifyProps{
		Msg:         i18n.For(i18n.FromContext(c.R.Context())),
		Token:       in.Token,
		CallbackURL: in.CallbackURL,
	}, nil
}, Verify)

// SendLink emails a magic link and always reports success, so the page does
// not tell whether an address has an account.
var SendLink = gx.Form(func(c *gx.Ctx, in *route.SendLink) error {
	w := webFrom(c.R.Context())
	if !w.cfg.MagicLinkEnabled() {
		return gx.NotFound()
	}
	address := strings.TrimSpace(in.Email)
	if a, err := mail.ParseAddress(address); err != nil || a.Address != address || len(address) > 254 {
		return c.Redirect(route.Login{Error: codeInvalidEmail})
	}
	if err := w.sendMagicLink(c.R, address); err != nil {
		var ae *apierr.Error
		if errors.As(err, &ae) {
			return c.Redirect(route.Login{Error: string(ae.Code)})
		}
		return err
	}
	return c.Redirect(route.Login{Sent: true})
}, Login)

// SignOut ends the session and returns to the sign-in page.
var SignOut = gx.Form(func(c *gx.Ctx, in *route.SignOut) error {
	w := webFrom(c.R.Context())
	sess, err := w.auth.Session(c.R.Context(), c.R)
	if err == nil && sess != nil {
		if err := w.auth.SignOut(c.R.Context(), sess); err != nil {
			return err
		}
	}
	w.auth.ClearSessionCookie(c.W)
	return c.Redirect(route.Login{})
}, SignOutButton)

// Routes lists every page and form of the slice.
var Routes = gx.Collect(LoginPage, VerifyPage, SendLink, SignOut)

// homeURL is the feed. Its route type arrives with the feed slice.
type homeURL struct{}

func (homeURL) URL() string { return "/" }

func loginError(msg i18n.Messages, code string) string {
	switch apierr.Code(code) {
	case apierr.CodeForbidden:
		return msg.LoginErrorNotAllowed
	case apierr.CodeInvalidToken:
		return msg.LoginErrorExpired
	case apierr.CodeRateLimited:
		return msg.LoginErrorRateLimited
	}
	if code == codeInvalidEmail {
		return msg.LoginErrorInvalidEmail
	}
	return msg.LoginErrorGeneric
}

// sendMagicLink runs auth-all's send endpoint in process, with the browser's
// origin and address, so its origin check and rate limit still apply.
//
// workaround: alternayte/auth-all#9
func (w *Web) sendMagicLink(r *http.Request, address string) error {
	body, err := json.Marshal(map[string]string{"email": address, "callbackURL": "/"})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "/api/auth/magic-link/send", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Host = r.Host
	req.RemoteAddr = r.RemoteAddr
	req.Header.Set("Content-Type", "application/json")
	for _, h := range []string{"Origin", "Sec-Fetch-Site", "User-Agent", "X-Forwarded-For", "X-Real-Ip"} {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}
	rec := &bufferedResponse{header: http.Header{}, status: http.StatusOK}
	w.authHTTP.ServeHTTP(rec, req)
	if rec.status < 300 {
		return nil
	}
	var out apierr.Body
	if json.Unmarshal(rec.body.Bytes(), &out) == nil && out.Error.Code != "" {
		return apierr.New(out.Error.Code, rec.status, out.Error.Message)
	}
	return errors.New("send magic link: auth answered " + http.StatusText(rec.status))
}
