package i18n

import (
	"testing"
	"time"
)

func TestFormatsFollowTheLocale(t *testing.T) {
	at := time.Date(2026, 10, 16, 4, 13, 0, 0, time.UTC)
	cases := []struct {
		got, want string
	}{
		{DateTime(EN, at), "16 Oct 2026, 04:13"},
		{DateTime(FR, at), "16 oct. 2026, 04:13"},
		{USD(EN, 0.2638), "$0.26"},
		{USD(FR, 0.2638), "0,26\u00a0$"},
		{USD(EN, 1234.5), "$1,234.50"},
		{USD(FR, 1234.5), "1\u202f234,50\u00a0$"},
		{Int(EN, 12345), "12,345"},
		{Int(FR, 12345), "12\u202f345"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestCountPicksThePluralOfTheLocale(t *testing.T) {
	cases := []struct {
		l    Locale
		n    int
		want string
	}{
		{EN, 0, "0 other"}, {EN, 1, "1 one"}, {EN, 2, "2 other"},
		{FR, 0, "0 one"}, {FR, 1, "1 one"}, {FR, 2, "2 other"},
	}
	for _, c := range cases {
		if got := Count(c.l, c.n, "%d one", "%d other"); got != c.want {
			t.Errorf("%s %d: %q", c.l, c.n, got)
		}
	}
}
