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
	months := monthsEN
	if l == FR {
		months = monthsFR
	}
	return fmt.Sprintf("%d %s %d, %02d:%02d", t.Day(), months[t.Month()-1], t.Year(), t.Hour(), t.Minute())
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
