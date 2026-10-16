package accounts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/alternayte/auth-all/email"

	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/platform"
)

const resendEndpoint = "https://api.resend.com/emails"

// resendSender delivers auth-all emails through Resend, in the locale of the
// request that caused them.
type resendSender struct {
	cfg      platform.Config
	endpoint string
	client   *http.Client
}

func newResendSender(cfg platform.Config, endpoint string, client *http.Client) *resendSender {
	return &resendSender{cfg: cfg, endpoint: endpoint, client: client}
}

func (s *resendSender) Send(ctx context.Context, m email.Message) error {
	if m.Intent != email.IntentMagicLink {
		return fmt.Errorf("send email: unsupported intent %q", m.Intent)
	}
	// The send endpoint never tells whether an address may sign in, so a
	// refused address gets no email and no error.
	if !s.cfg.EmailAllowed(m.To) {
		return nil
	}
	msg := i18n.For(i18n.FromContext(ctx))
	minutes := int(math.Ceil(time.Until(m.ExpiresAt).Minutes()))
	return s.post(ctx, map[string]any{
		"from":    s.cfg.EmailFrom,
		"to":      []string{m.To},
		"subject": msg.MagicLinkSubject,
		"text":    msg.MagicLinkText(m.URL, minutes),
	})
}

func (s *resendSender) post(ctx context.Context, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.ResendAPIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("send email: resend answered %d: %s", resp.StatusCode, detail)
	}
	return nil
}
