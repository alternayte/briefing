package accounts

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
	// Timezone is an IANA zone name, empty until the browser sends one; empty
	// means UTC. The reader's day, for the feed, the daily budget and the
	// review cap, starts at midnight in this zone.
	Timezone string `db:"timezone,default=''"`
}

func newProfile(userID uuid.UUID, displayName string, locale i18n.Locale, dailyBudget int) *Profile {
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

// Location returns the reader's time zone, or UTC when the name is unknown.
func (p *Profile) Location() *time.Location { return Location(p.Timezone) }

// Location loads an IANA zone, or UTC when the name is empty or unknown.
func Location(name string) *time.Location {
	if loc, err := time.LoadLocation(name); err == nil && name != "" {
		return loc
	}
	return time.UTC
}

// LocalDay returns the reader's date at t, as YYYY-MM-DD.
func LocalDay(t time.Time, loc *time.Location) string { return t.In(loc).Format(time.DateOnly) }

// StartOfDay returns the reader's last local midnight at or before t.
func StartOfDay(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}
