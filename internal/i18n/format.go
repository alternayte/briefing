package i18n

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var monthsEN = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
var monthsFR = []string{"janv.", "févr.", "mars", "avr.", "mai", "juin", "juil.", "août", "sept.", "oct.", "nov.", "déc."}

// DateTime formats a time as day, short month, year and 24-hour time, in UTC.
func DateTime(l Locale, t time.Time) string {
	t = t.UTC()
	return fmt.Sprintf("%d %s %d, %02d:%02d", t.Day(), months(l)[t.Month()-1], t.Year(), t.Hour(), t.Minute())
}

// DateTimeIn formats a time as day, short month and 24-hour time in loc.
func DateTimeIn(l Locale, t time.Time, loc *time.Location) string {
	t = t.In(loc)
	return fmt.Sprintf("%d %s %02d:%02d", t.Day(), months(l)[t.Month()-1], t.Hour(), t.Minute())
}

// Ago says how long before now t was: minutes, then hours within a day,
// "yesterday" on the reader's previous day, then a short date.
func Ago(l Locale, t, now time.Time, loc *time.Location) string {
	m := For(l)
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return m.AgoJustNow
	case d < time.Hour:
		return fmt.Sprintf(m.AgoMinutes, int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf(m.AgoHours, int(d/time.Hour))
	}
	t, now = t.In(loc), now.In(loc)
	if y, mo, day := now.AddDate(0, 0, -1).Date(); t.Year() == y && t.Month() == mo && t.Day() == day {
		return m.AgoYesterday
	}
	return fmt.Sprintf("%d %s", t.Day(), months(l)[t.Month()-1])
}

func months(l Locale) []string {
	if l == FR {
		return monthsFR
	}
	return monthsEN
}

// Int formats a whole number with the locale's thousands separator.
func Int(l Locale, n int64) string {
	return group(strconv.FormatInt(n, 10), thousands(l))
}

// USD formats an amount in US dollars with two decimals.
func USD(l Locale, amount float64) string {
	s := strconv.FormatFloat(amount, 'f', 2, 64)
	whole, frac, _ := strings.Cut(s, ".")
	whole = group(whole, thousands(l))
	if l == FR {
		return whole + "," + frac + " $"
	}
	return "$" + whole + "." + frac
}

func thousands(l Locale) string {
	if l == FR {
		// A narrow no-break space, as French typography sets it.
		return " "
	}
	return ","
}

func group(digits, sep string) string {
	neg := strings.HasPrefix(digits, "-")
	digits = strings.TrimPrefix(digits, "-")
	var b strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteString(sep)
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// Count words a number with the singular or plural format of the locale:
// French takes the singular for 0 and 1, English for 1 only.
func Count(l Locale, n int, one, other string) string {
	singular := n == 1
	if l == FR {
		singular = n < 2
	}
	if singular {
		return fmt.Sprintf(one, n)
	}
	return fmt.Sprintf(other, n)
}
