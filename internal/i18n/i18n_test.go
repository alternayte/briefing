package i18n

import (
	"reflect"
	"testing"
)

func TestEveryLocaleFillsEveryMessage(t *testing.T) {
	for _, l := range Locales {
		v := reflect.ValueOf(For(l))
		for i := range v.NumField() {
			if v.Field(i).String() == "" {
				t.Errorf("%s: %s is empty", l, v.Type().Field(i).Name)
			}
		}
	}
}

func TestFromAcceptLanguage(t *testing.T) {
	cases := map[string]Locale{
		"":                        EN,
		"fr-FR,fr;q=0.9,en;q=0.8": FR,
		"en-GB,en;q=0.9,fr;q=0.8": EN,
		"de-DE,fr;q=0.7,en;q=0.5": FR,
		"de-DE,es;q=0.9":          EN,
		"en;q=0.2,FR;q=0.8":       FR,
		"fr;q=0,en":               EN,
		"*":                       EN,
		"garbage;;q=x,fr-CA":      FR,
	}
	for header, want := range cases {
		if got := FromAcceptLanguage(header); got != want {
			t.Errorf("%q: got %s, want %s", header, got, want)
		}
	}
}

func TestParse(t *testing.T) {
	if l, ok := Parse("fr"); !ok || l != FR {
		t.Errorf("fr = %s %v", l, ok)
	}
	if _, ok := Parse("de"); ok {
		t.Error("de must not parse")
	}
}
