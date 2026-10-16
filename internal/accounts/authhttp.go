package accounts

import (
	"bytes"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/alternayte/auth-all/apierr"
)

// browserErrors sends the browser to /login with the error code when an
// auth-all endpoint that a browser navigates to answers with an error.
//
// workaround: alternayte/auth-all#8
func browserErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !reachedByBrowser(r) {
			next.ServeHTTP(w, r)
			return
		}
		rec := &bufferedResponse{header: http.Header{}, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if rec.status < 400 {
			rec.flushTo(w)
			return
		}
		var body apierr.Body
		code := string(apierr.CodeInternal)
		if json.Unmarshal(rec.body.Bytes(), &body) == nil && body.Error.Code != "" {
			code = string(body.Error.Code)
		}
		http.Redirect(w, r, "/login?error="+url.QueryEscape(code), http.StatusSeeOther)
	})
}

func reachedByBrowser(r *http.Request) bool {
	p := r.URL.Path
	if r.Method == http.MethodGet && strings.HasPrefix(p, "/api/auth/oauth/") && strings.HasSuffix(p, "/callback") {
		return true
	}
	if r.Method == http.MethodPost && p == "/api/auth/magic-link/verify" {
		mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		return mt == "application/x-www-form-urlencoded"
	}
	return false
}

type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferedResponse) Header() http.Header         { return b.header }
func (b *bufferedResponse) Write(p []byte) (int, error) { return b.body.Write(p) }
func (b *bufferedResponse) WriteHeader(status int)      { b.status = status }

func (b *bufferedResponse) flushTo(w http.ResponseWriter) {
	for k, v := range b.header {
		w.Header()[k] = v
	}
	w.WriteHeader(b.status)
	_, _ = w.Write(b.body.Bytes())
}
