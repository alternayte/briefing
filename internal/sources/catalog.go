package sources

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/alternayte/briefing/internal/i18n"
	"github.com/alternayte/briefing/internal/platform"
)

// Entry is one validated catalog line.
type Entry struct {
	Kind         SourceKind
	URL          string
	Title        string
	Tier         int
	Language     string
	PollInterval time.Duration
}

type rawEntry struct {
	Kind        SourceKind `yaml:"kind"`
	URL         string     `yaml:"url"`
	Title       string     `yaml:"title"`
	Tier        int        `yaml:"tier"`
	Language    string     `yaml:"language"`
	PollMinutes int        `yaml:"poll_minutes"`
}

var handlePattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}$`)

// ParseCatalog reads a catalog document and reports every invalid entry.
func ParseCatalog(doc []byte, t platform.Tuning) ([]Entry, error) {
	dec := yaml.NewDecoder(bytes.NewReader(doc))
	dec.KnownFields(true)
	var raw []rawEntry
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	var errs []error
	seen := map[string]bool{}
	out := make([]Entry, 0, len(raw))
	for i, r := range raw {
		e, err := r.entry(t)
		if err != nil {
			errs = append(errs, fmt.Errorf("entry %d (%s): %w", i+1, r.URL, err))
			continue
		}
		key := string(e.Kind) + " " + e.URL
		if seen[key] {
			errs = append(errs, fmt.Errorf("entry %d: duplicate %s", i+1, key))
		}
		seen[key] = true
		out = append(out, e)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("catalog: %w", errors.Join(errs...))
	}
	return out, nil
}

func (r rawEntry) entry(t platform.Tuning) (Entry, error) {
	e := Entry{Kind: r.Kind, URL: strings.TrimSpace(r.URL), Title: strings.TrimSpace(r.Title), Tier: r.Tier, Language: r.Language}
	if e.Title == "" {
		return e, errors.New("title is required")
	}
	if e.Tier < 1 || e.Tier > 3 {
		return e, fmt.Errorf("tier must be 1 to 3, got %d", e.Tier)
	}
	if e.Language == "" {
		e.Language = string(i18n.Locales[0])
	}
	if _, ok := i18n.Parse(e.Language); !ok {
		return e, fmt.Errorf("unsupported language %q", e.Language)
	}
	switch e.Kind {
	case KindRSS, KindHN:
		u, err := url.Parse(e.URL)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return e, fmt.Errorf("url must be an absolute https URL")
		}
		e.PollInterval = t.Poll.RSS
		if e.Kind == KindHN {
			e.PollInterval = t.Poll.HN
		}
	case KindBluesky:
		e.URL = strings.ToLower(strings.TrimPrefix(e.URL, "@"))
		if !handlePattern.MatchString(e.URL) {
			return e, fmt.Errorf("url must be a Bluesky handle")
		}
		e.PollInterval = t.Poll.Bluesky
	default:
		return e, fmt.Errorf("unknown kind %q", e.Kind)
	}
	if r.PollMinutes > 0 {
		e.PollInterval = time.Duration(r.PollMinutes) * time.Minute
	}
	return e, nil
}
