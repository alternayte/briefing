// Package i18n holds the UI message catalogs and the locale rules.
package i18n

import (
	"cmp"
	"context"
	"slices"
	"strconv"
	"strings"
)

type Locale string

const (
	EN Locale = "en"
	FR Locale = "fr"
)

// Locales lists every supported locale; the first is the default.
var Locales = []Locale{EN, FR}

// Parse returns the supported locale named by s.
func Parse(s string) (Locale, bool) {
	l := Locale(strings.ToLower(strings.TrimSpace(s)))
	return l, slices.Contains(Locales, l)
}

// FromAcceptLanguage picks the supported locale the browser prefers most, and
// the default when it names none.
func FromAcceptLanguage(header string) Locale {
	type pref struct {
		locale Locale
		q      float64
		order  int
	}
	var prefs []pref
	for i, part := range strings.Split(header, ",") {
		tag, params, _ := strings.Cut(part, ";")
		base, _, _ := strings.Cut(strings.TrimSpace(tag), "-")
		l, ok := Parse(base)
		if !ok {
			continue
		}
		q := 1.0
		for p := range strings.SplitSeq(params, ";") {
			if v, found := strings.CutPrefix(strings.TrimSpace(p), "q="); found {
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					q = f
				}
			}
		}
		if q > 0 {
			prefs = append(prefs, pref{l, q, i})
		}
	}
	if len(prefs) == 0 {
		return Locales[0]
	}
	best := slices.MinFunc(prefs, func(a, b pref) int {
		return cmp.Or(cmp.Compare(b.q, a.q), cmp.Compare(a.order, b.order))
	})
	return best.locale
}

// For returns the catalog of l, or of the default locale when l is unknown.
func For(l Locale) Messages {
	if l == FR {
		return fr
	}
	return en
}

type localeKey struct{}

// WithLocale returns a context that carries the locale of the request.
func WithLocale(ctx context.Context, l Locale) context.Context {
	return context.WithValue(ctx, localeKey{}, l)
}

// FromContext returns the locale of the request, or the default.
func FromContext(ctx context.Context) Locale {
	if l, ok := ctx.Value(localeKey{}).(Locale); ok {
		return l
	}
	return Locales[0]
}
