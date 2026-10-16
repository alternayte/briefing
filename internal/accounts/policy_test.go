package accounts

import (
	"context"
	"testing"

	"github.com/alternayte/auth-all/hook"
	"github.com/alternayte/auth-all/store"

	"github.com/alternayte/briefing/internal/platform"
)

func TestAllowlistRefusesUnlistedEmailOnSignUpAndSignIn(t *testing.T) {
	cfg := platform.Config{AllowedEmails: []string{"ann@example.com"}}
	h := hook.New(nil)
	applyAllowlist(h, cfg)
	ctx := context.Background()

	for email, allowed := range map[string]bool{"Ann@Example.com": true, "eve@example.com": false} {
		u := &store.User{Email: email}
		createErr := h.RunBeforeUserCreate(ctx, &hook.UserCreate{User: u})
		sessionErr := h.RunBeforeSessionCreate(ctx, &hook.SessionCreate{User: u, Session: &store.Session{}})
		if allowed && (createErr != nil || sessionErr != nil) {
			t.Errorf("%s refused: %v %v", email, createErr, sessionErr)
		}
		if !allowed && (createErr == nil || sessionErr == nil) {
			t.Errorf("%s accepted", email)
		}
	}
}

func TestOpenSignUpAcceptsEveryone(t *testing.T) {
	h := hook.New(nil)
	applyAllowlist(h, platform.Config{})
	if err := h.RunBeforeUserCreate(context.Background(), &hook.UserCreate{User: &store.User{Email: "x@example.com"}}); err != nil {
		t.Fatal(err)
	}
}
