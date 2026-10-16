package ingest

import (
	"html"
	"strings"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"
)

// plainText drops markup, decodes entities, collapses white space and cuts the
// result at max runes on a word boundary.
func plainText(s string, max int) string {
	var b strings.Builder
	z := xhtml.NewTokenizer(strings.NewReader(s))
	skip := 0
	for {
		switch z.Next() {
		case xhtml.ErrorToken:
			return cut(strings.Join(strings.Fields(b.String()), " "), max)
		case xhtml.StartTagToken:
			if name, _ := z.TagName(); string(name) == "script" || string(name) == "style" {
				skip++
			}
			b.WriteByte(' ')
		case xhtml.EndTagToken:
			if name, _ := z.TagName(); (string(name) == "script" || string(name) == "style") && skip > 0 {
				skip--
			}
			b.WriteByte(' ')
		case xhtml.TextToken:
			if skip == 0 {
				b.WriteString(html.UnescapeString(string(z.Text())))
			}
		}
	}
}

func cut(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)[:max-1]
	if i := strings.LastIndexByte(string(r), ' '); i > len(string(r))/2 {
		return string(r)[:i] + "…"
	}
	return string(r) + "…"
}
