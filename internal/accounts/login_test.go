package accounts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/platform"
)

func withWeb(r *http.Request, cfg platform.Config, l i18n.Locale) *http.Request {
	ctx := context.WithValue(r.Context(), webKey{}, &Web{cfg: cfg})
	return r.WithContext(i18n.WithLocale(ctx, l))
}

func TestLoginPageSpeaksTheRequestLocale(t *testing.T) {
	rec := httptest.NewRecorder()
	req := withWeb(httptest.NewRequest("GET", "/login?error=FORBIDDEN", nil), platform.Config{ResendAPIKey: "re_x"}, i18n.FR)
	LoginPage.ServeHTTP(rec, req)
	body := rec.Body.String()
	fr := i18n.For(i18n.FR)
	for _, want := range []string{fr.LoginTitle, fr.LoginGitHub, fr.LoginEmailSend, fr.LoginErrorNotAllowed} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %q", want)
		}
	}
}

func TestLoginPageHidesEmailWhenMagicLinkIsOff(t *testing.T) {
	rec := httptest.NewRecorder()
	LoginPage.ServeHTTP(rec, withWeb(httptest.NewRequest("GET", "/login", nil), platform.Config{}, i18n.EN))
	if strings.Contains(rec.Body.String(), i18n.For(i18n.EN).LoginEmailSend) {
		t.Error("email form shown without RESEND_API_KEY")
	}
}

func TestSendLinkRejectsAnInvalidAddressWithARedirect(t *testing.T) {
	for _, address := range []string{"", "not-an-address", "Ann <ann@example.com>"} {
		form := url.Values{"Email": {address}}
		req := httptest.NewRequest("POST", "/login/link", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		rec := httptest.NewRecorder()
		SendLink.ServeHTTP(rec, withWeb(req, platform.Config{ResendAPIKey: "re_x"}, i18n.EN))
		if loc := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || loc != "/login?error=INVALID_EMAIL" {
			t.Errorf("%q: got %d %q", address, rec.Code, loc)
		}
	}
}

func TestLoginErrorCodesMapToMessages(t *testing.T) {
	en := i18n.For(i18n.EN)
	cases := map[string]string{
		"FORBIDDEN":           en.LoginErrorNotAllowed,
		"INVALID_TOKEN":       en.LoginErrorExpired,
		"RATE_LIMITED":        en.LoginErrorRateLimited,
		"INVALID_EMAIL":       en.LoginErrorInvalidEmail,
		"OAUTH_STATE_INVALID": en.LoginErrorGeneric,
	}
	for code, want := range cases {
		if got := loginError(en, code); got != want {
			t.Errorf("%s: %q", code, got)
		}
	}
}
