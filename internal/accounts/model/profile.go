// Package model holds the drel models of the accounts slice. They live apart
// from the slice code; workaround: alternayte/drel#10
package model

import (
	"time"

	"github.com/alternayte/drel"
	"github.com/google/uuid"

	"github.com/alternayte/briefing/internal/i18n"
)

// Profile is the app side of an auth-all user. Its id is the user id.
type Profile struct {
	drel.Model[uuid.UUID]
	DisplayName string     `db:"display_name"`
	Locale      string     `db:"locale"`
	DailyBudget int        `db:"daily_budget"`
	OnboardedAt *time.Time `db:"onboarded_at"`
}

func NewProfile(userID uuid.UUID, displayName string, locale i18n.Locale, dailyBudget int) *Profile {
	p := &Profile{DisplayName: displayName, Locale: string(locale), DailyBudget: dailyBudget}
	p.SetID(userID)
	return p
}

// UserLocale returns the profile locale, or the default when it is unknown.
func (p *Profile) UserLocale() i18n.Locale {
	if l, ok := i18n.Parse(p.Locale); ok {
		return l
	}
	return i18n.Locales[0]
}

func (p *Profile) SetLocale(l i18n.Locale) { p.Locale = string(l) }
