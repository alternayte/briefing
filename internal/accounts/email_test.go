package accounts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alternayte/auth-all/email"

	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/platform"
)

type sentEmail struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
}

func resendStub(t *testing.T) (*httptest.Server, *[]sentEmail) {
	t.Helper()
	var sent []sentEmail
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer re_test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var m sentEmail
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			t.Error(err)
		}
		sent = append(sent, m)
		_, _ = w.Write([]byte(`{"id":"49a3999c-0ce1-4ea6-ab68-afcd6dc2e794"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &sent
}

func magicLink(to string) email.Message {
	return email.Message{
		Intent:    email.IntentMagicLink,
		To:        to,
		URL:       "https://briefing.example.com/login/verify?token=abc",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
}

func TestMagicLinkEmailUsesRequestLocale(t *testing.T) {
	srv, sent := resendStub(t)
	cfg := platform.Config{ResendAPIKey: "re_test", EmailFrom: "Briefing <briefing@example.com>"}
	s := newResendSender(cfg, srv.URL, srv.Client())

	ctx := i18n.WithLocale(context.Background(), i18n.FR)
	if err := s.Send(ctx, magicLink("ann@example.com")); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 1 {
		t.Fatalf("sent %d emails", len(*sent))
	}
	m := (*sent)[0]
	if m.Subject != i18n.For(i18n.FR).MagicLinkSubject || m.To[0] != "ann@example.com" || m.From != cfg.EmailFrom {
		t.Errorf("email = %+v", m)
	}
	if !strings.Contains(m.Text, "https://briefing.example.com/login/verify?token=abc") || !strings.Contains(m.Text, "15 minutes") {
		t.Errorf("text = %q", m.Text)
	}
}

func TestMagicLinkNotSentOutsideAllowlist(t *testing.T) {
	srv, sent := resendStub(t)
	cfg := platform.Config{ResendAPIKey: "re_test", EmailFrom: "b@example.com", AllowedEmails: []string{"ann@example.com"}}
	if err := newResendSender(cfg, srv.URL, srv.Client()).Send(context.Background(), magicLink("eve@example.com")); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 0 {
		t.Errorf("sent %d emails to an address outside the allowlist", len(*sent))
	}
}

func TestResendFailureIsAnError(t *testing.T) {
	srv, _ := resendStub(t)
	cfg := platform.Config{ResendAPIKey: "wrong", EmailFrom: "b@example.com"}
	if err := newResendSender(cfg, srv.URL, srv.Client()).Send(context.Background(), magicLink("ann@example.com")); err == nil {
		t.Error("want an error")
	}
}
