package write

import (
	"strings"
	"testing"

	"github.com/alternayte/briefing/internal/platform"
)

func validOutput() Output {
	return Output{
		Headline:     "Probe lands on an active comet",
		Summary:      strings.Repeat("word ", 60),
		WhyItMatters: "It matters.",
		WhatIsNew:    "It is new.",
		Claims:       []OutClaim{{Text: "It landed.", ItemIDs: []string{"101"}}},
		Concepts:     []OutConcept{{Name: "Soft landing", Definition: "A landing at low speed."}, {Name: "Active comet", Definition: "A comet that releases gas."}},
		Review: []OutReview{
			{Question: "Q1?", Choices: []string{"a", "b", "c", "d"}, AnswerIndex: 1, Explanation: "Because."},
			{Question: "Q2?", Choices: []string{"a", "b", "c", "d"}, AnswerIndex: 3, Explanation: "Because."},
		},
	}
}

func TestValidateAcceptsAGoodEntry(t *testing.T) {
	if err := validate(validOutput(), map[string]bool{"101": true}, platform.DefaultTuning().Write); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := map[string]func(o *Output){
		"unsupplied item id":  func(o *Output) { o.Claims[0].ItemIDs = []string{"999"} },
		"claim without items": func(o *Output) { o.Claims[0].ItemIDs = nil },
		"no claims":           func(o *Output) { o.Claims = nil },
		"long summary":        func(o *Output) { o.Summary = strings.Repeat("word ", 121) },
		"empty headline":      func(o *Output) { o.Headline = " " },
		"empty why":           func(o *Output) { o.WhyItMatters = "" },
		"one concept":         func(o *Output) { o.Concepts = o.Concepts[:1] },
		"empty definition":    func(o *Output) { o.Concepts[0].Definition = "" },
		"one question":        func(o *Output) { o.Review = o.Review[:1] },
		"three choices":       func(o *Output) { o.Review[0].Choices = []string{"a", "b", "c"} },
		"answer out of range": func(o *Output) { o.Review[1].AnswerIndex = 4 },
		"negative answer":     func(o *Output) { o.Review[1].AnswerIndex = -1 },
		"empty explanation":   func(o *Output) { o.Review[0].Explanation = "" },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			o := validOutput()
			breakIt(&o)
			if err := validate(o, map[string]bool{"101": true}, platform.DefaultTuning().Write); err == nil {
				t.Error("want an error")
			}
		})
	}
}

func TestConfidenceRules(t *testing.T) {
	cases := []struct {
		publishers, textItems int
		want                  Confidence
	}{
		{3, 2, ConfidenceHigh},
		{2, 2, ConfidenceHigh},
		{2, 1, ConfidenceMedium},
		{1, 1, ConfidenceMedium},
		{1, 0, ConfidenceLow},
	}
	for _, c := range cases {
		if got := confidence(c.publishers, c.textItems); got != c.want {
			t.Errorf("%d publishers, %d text items: %s", c.publishers, c.textItems, got)
		}
	}
}

func TestParseOutputToleratesCodeFences(t *testing.T) {
	o, err := parseOutput("```json\n{\"headline\":\"H\"}\n```")
	if err != nil || o.Headline != "H" {
		t.Errorf("%+v %v", o, err)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Soft landing":            "soft-landing",
		"  Heavy-water ratio ":    "heavy-water-ratio",
		"Retrieval-Augmented Gen": "retrieval-augmented-gen",
		"Café au lait (1999)":     "cafe-au-lait-1999",
	} {
		if got := slug(in); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
}
