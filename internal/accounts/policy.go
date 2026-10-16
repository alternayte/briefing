package accounts

import (
	"context"

	"github.com/alternayte/auth-all/apierr"
	"github.com/alternayte/auth-all/hook"

	"github.com/alternayte/briefing/internal/platform"
)

var errNotAllowed = apierr.ErrForbidden.WithMessage("This email address cannot sign in to this instance.")

// applyAllowlist refuses sign-up and sign-in for addresses outside
// ALLOWED_EMAILS. Sign-in is checked too, so removing an address takes effect
// at the next sign-in.
func applyAllowlist(h *hook.Hooks, cfg platform.Config) {
	h.OnBeforeUserCreate(func(_ context.Context, ev *hook.UserCreate) error {
		if !cfg.EmailAllowed(ev.User.Email) {
			return errNotAllowed
		}
		return nil
	})
	h.OnBeforeSessionCreate(func(_ context.Context, ev *hook.SessionCreate) error {
		if !cfg.EmailAllowed(ev.User.Email) {
			return errNotAllowed
		}
		return nil
	})
}
