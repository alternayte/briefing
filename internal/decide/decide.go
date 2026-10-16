// Package decide asks decision models typed questions through OpenRouter.
package decide

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/alternayte/briefing/internal/platform"
)

// Question is one typed question. Build it with Noul, Choice or Score.
type Question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria"`
}

// Questions maps a question name to its question.
type Questions map[string]Question

// Noul asks for the probability that a condition holds.
func Noul(instructions, whenTrue, whenFalse string) Question {
	return Question{Type: "noul", Instructions: instructions, Criteria: map[string]string{"true": whenTrue, "false": whenFalse}}
}

// Choice asks for one of the named options.
func Choice(instructions string, options ...string) Question {
	c := make(map[string]any, len(options))
	for _, o := range options {
		c[o] = nil
	}
	return Question{Type: "choice", Instructions: instructions, Criteria: c}
}

// Score asks for a position on ordered levels, lowest first. The answer's
// Score is the probability-weighted level index, from 0 to len(levels)-1.
func Score(instructions string, levels ...string) Question {
	return Question{Type: "score", Instructions: instructions, Criteria: levels}
}

// Answer is one typed answer; only the field of its type is set.
type Answer struct {
	Type          string             `json:"type"`
	Noul          float64            `json:"noul"`
	Choice        string             `json:"choice"`
	Score         float64            `json:"score"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
}

type Answers map[string]Answer

// Decider asks the configured decision model.
type Decider struct {
	or       *platform.OpenRouter
	path     string
	model    string
	maxState int
}

func New(or *platform.OpenRouter, cfg platform.Config, t platform.Tuning) *Decider {
	path := "/api/alpha/decisions"
	if cfg.DecideAPI == platform.DecideAPISystemOne {
		path = "/api/v1/systemone"
	}
	return &Decider{or: or, path: path, model: cfg.DecideModel, maxState: t.Triage.MaxStateChars}
}

// Model is the decision model id, as recorded with each triage.
func (d *Decider) Model() string { return d.model }

// Ask sends one request with every question. The state is cut to the cap, so
// a decision model never receives more than the cap.
func (d *Decider) Ask(ctx context.Context, stage, state string, qs Questions) (Answers, error) {
	req := struct {
		Model     string    `json:"model"`
		State     string    `json:"state"`
		Questions Questions `json:"questions"`
	}{d.model, capRunes(state, d.maxState), qs}
	var resp struct {
		Answers Answers `json:"answers"`
	}
	if err := d.or.Post(ctx, d.path, stage, d.model, req, &resp); err != nil {
		return nil, fmt.Errorf("decide %s: %w", stage, err)
	}
	for name, q := range qs {
		a, ok := resp.Answers[name]
		if !ok || a.Type != q.Type {
			return nil, fmt.Errorf("decide %s: no %s answer for %q", stage, q.Type, name)
		}
	}
	return resp.Answers, nil
}

func capRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}
