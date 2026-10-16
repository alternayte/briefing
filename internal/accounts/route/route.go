// Package route holds the route types of the accounts slice. A route package
// holds only route types, so every slice can link to every other slice.
package route

import "github.com/alternayte/gx"

// Login is the sign-in page. Error is an auth-all error code; Sent reports
// that a magic link went out.
type Login struct {
	gx.Route `GET /login`
	Error    string `query:"error"`
	Sent     bool   `query:"sent"`
}

// LoginVerify confirms a magic link before auth-all consumes its token.
type LoginVerify struct {
	gx.Route    `GET /login/verify`
	Token       string `query:"token"`
	CallbackURL string `query:"callbackURL"`
}

// SendLink asks for a magic link. The handler validates the address, so a
// failed submit redirects instead of rendering the form with empty props.
type SendLink struct {
	gx.Route `POST /login/link`
	Email    string
}

func (in *SendLink) Rules() gx.Rules { return gx.Rules{} }

// SignOut ends the current session.
type SignOut struct {
	gx.Route `POST /logout`
}

func (in *SignOut) Rules() gx.Rules { return gx.Rules{} }
