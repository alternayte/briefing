package accounts

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alternayte/auth-all/apierr"
)

func TestBrowserErrorsRedirectToLogin(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/oauth/github/callback", "/api/auth/magic-link/verify":
			apierr.Write(w, apierr.ErrForbidden)
		case "/api/auth/oauth/github/ok":
			http.Redirect(w, r, "/", http.StatusFound)
		default:
			apierr.Write(w, apierr.ErrUnauthorized)
		}
	})
	h := browserErrors(inner)

	cases := []struct {
		method, path, contentType string
		wantStatus                int
		wantLocation              string
	}{
		{"GET", "/api/auth/oauth/github/callback?code=x", "", http.StatusSeeOther, "/login?error=FORBIDDEN"},
		{"POST", "/api/auth/magic-link/verify", "application/x-www-form-urlencoded", http.StatusSeeOther, "/login?error=FORBIDDEN"},
		{"POST", "/api/auth/magic-link/verify", "application/json", http.StatusForbidden, ""},
		{"GET", "/api/auth/oauth/github/ok", "", http.StatusFound, "/"},
		{"GET", "/api/auth/session", "", http.StatusUnauthorized, ""},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, strings.NewReader(""))
		if c.contentType != "" {
			req.Header.Set("Content-Type", c.contentType)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.wantStatus || rec.Header().Get("Location") != c.wantLocation {
			t.Errorf("%s %s: got %d %q, want %d %q", c.method, c.path, rec.Code, rec.Header().Get("Location"), c.wantStatus, c.wantLocation)
		}
	}
}
