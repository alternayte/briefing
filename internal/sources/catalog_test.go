package sources

import (
	"strings"
	"testing"

	"github.com/alternayte/briefing/catalog"
	"github.com/alternayte/briefing/internal/platform"
)

func TestShippedCatalogIsValid(t *testing.T) {
	entries, err := ParseCatalog(catalog.Sources, platform.DefaultTuning())
	if err != nil {
		t.Fatal(err)
	}
	count := map[SourceKind]int{}
	for _, e := range entries {
		count[e.Kind]++
	}
	if count[KindRSS] < 25 || count[KindHN] != 1 || count[KindBluesky] != 10 {
		t.Errorf("catalog counts = %v", count)
	}
}

func TestParseCatalogRejectsBadEntries(t *testing.T) {
	cases := map[string]string{
		"unknown kind":      `[{kind: reddit, title: x, tier: 1, url: "https://reddit.com/r/x.rss"}]`,
		"plain http feed":   `[{kind: rss, title: x, tier: 1, url: "http://example.com/feed"}]`,
		"tier out of range": `[{kind: rss, title: x, tier: 4, url: "https://example.com/feed"}]`,
		"missing title":     `[{kind: rss, tier: 1, url: "https://example.com/feed"}]`,
		"bad handle":        `[{kind: bluesky, title: x, tier: 1, url: "not a handle"}]`,
		"unknown language":  `[{kind: rss, title: x, tier: 1, language: de, url: "https://example.com/feed"}]`,
		"duplicate":         `[{kind: rss, title: a, tier: 1, url: "https://example.com/feed"}, {kind: rss, title: b, tier: 2, url: "https://example.com/feed"}]`,
		"unknown field":     `[{kind: rss, title: x, tier: 1, url: "https://example.com/feed", colour: red}]`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseCatalog([]byte(doc), platform.DefaultTuning()); err == nil {
				t.Error("want an error")
			}
		})
	}
}

func TestParseCatalogAppliesDefaults(t *testing.T) {
	tu := platform.DefaultTuning()
	entries, err := ParseCatalog([]byte(`
- {kind: bluesky, title: X, tier: 2, url: Example.COM}
- {kind: rss, title: Y, tier: 1, url: "https://example.com/feed", poll_minutes: 5}
`), tu)
	if err != nil {
		t.Fatal(err)
	}
	if e := entries[0]; e.Language != "en" || e.URL != "example.com" || e.PollInterval != tu.Poll.Bluesky {
		t.Errorf("bluesky entry = %+v", e)
	}
	if e := entries[1]; e.PollInterval.Minutes() != 5 || !strings.HasPrefix(e.URL, "https://") {
		t.Errorf("rss entry = %+v", e)
	}
}
