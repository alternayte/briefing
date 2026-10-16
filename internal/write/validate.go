package write

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/alternayte/briefing/internal/platform"
)

// Output is the JSON the writer model returns.
type Output struct {
	Headline     string       `json:"headline"`
	Summary      string       `json:"summary"`
	WhyItMatters string       `json:"why_it_matters"`
	WhatIsNew    string       `json:"what_is_new"`
	Claims       []OutClaim   `json:"claims"`
	Concepts     []OutConcept `json:"concepts"`
	Review       []OutReview  `json:"review"`
}

type OutClaim struct {
	Text    string   `json:"text"`
	ItemIDs []string `json:"item_ids"`
}

type OutConcept struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

type OutReview struct {
	Question    string   `json:"question"`
	Choices     []string `json:"choices"`
	AnswerIndex int      `json:"answer_index"`
	Explanation string   `json:"explanation"`
}

func parseOutput(content string) (Output, error) {
	s := strings.TrimSpace(content)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	var o Output
	if err := json.Unmarshal([]byte(s), &o); err != nil {
		return o, fmt.Errorf("writer output is not JSON: %w", err)
	}
	return o, nil
}

// validate checks the rules a written entry must meet before it is stored.
func validate(o Output, supplied map[string]bool, t platform.WriteTuning) error {
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }
	blank := func(s string) bool { return strings.TrimSpace(s) == "" }

	if blank(o.Headline) || blank(o.Summary) || blank(o.WhyItMatters) || blank(o.WhatIsNew) {
		fail("a field is empty")
	}
	if n := len(strings.Fields(o.Summary)); n > t.SummaryRejectWords {
		fail("summary has %d words, more than %d", n, t.SummaryRejectWords)
	}
	if len(o.Claims) == 0 {
		fail("no claims")
	}
	for i, c := range o.Claims {
		if blank(c.Text) || len(c.ItemIDs) == 0 {
			fail("claim %d has no text or no items", i)
		}
		for _, id := range c.ItemIDs {
			if !supplied[id] {
				fail("claim %d cites item %q, which was not supplied", i, id)
			}
		}
	}
	if n := len(o.Concepts); n < t.MinConcepts || n > t.MaxConcepts {
		fail("%d concepts, want %d to %d", n, t.MinConcepts, t.MaxConcepts)
	}
	for i, c := range o.Concepts {
		if blank(c.Name) || blank(c.Definition) || slug(c.Name) == "" {
			fail("concept %d is incomplete", i)
		}
	}
	if len(o.Review) != t.ReviewQuestions {
		fail("%d review questions, want %d", len(o.Review), t.ReviewQuestions)
	}
	for i, r := range o.Review {
		if blank(r.Question) || blank(r.Explanation) || len(r.Choices) != t.ReviewChoices {
			fail("review question %d is incomplete", i)
		}
		if r.AnswerIndex < 0 || r.AnswerIndex >= len(r.Choices) {
			fail("review question %d answer index %d is out of range", i, r.AnswerIndex)
		}
		for _, c := range r.Choices {
			if blank(c) {
				fail("review question %d has an empty choice", i)
			}
		}
	}
	return errors.Join(errs...)
}

// confidence follows the evidence: two or more publishers and two or more
// items with text is high; any item with text is medium; otherwise low.
func confidence(publishers, textItems int) Confidence {
	switch {
	case publishers >= 2 && textItems >= 2:
		return ConfidenceHigh
	case textItems >= 1:
		return ConfidenceMedium
	}
	return ConfidenceLow
}

// slug is the stable key of a concept name: lower case ASCII words joined by
// hyphens.
func slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range norm.NFKD.String(strings.ToLower(name)) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		default:
			dash = true
		}
	}
	return b.String()
}
