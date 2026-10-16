package platform

import "testing"

func TestDefaultTuningIsValid(t *testing.T) {
	if err := DefaultTuning().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestTuningValidateRejectsBrokenInvariants(t *testing.T) {
	cases := map[string]func(*Tuning){
		"score weights do not sum to 1": func(tu *Tuning) { tu.Score.Relevance += 0.1 },
		"join band inverted":            func(tu *Tuning) { tu.Cluster.JoinLow = tu.Cluster.JoinHigh + 0.01 },
		"review intervals not rising":   func(tu *Tuning) { tu.Review.IntervalsDays = []int{1, 3, 3} },
		"taxonomy without other":        func(tu *Tuning) { tu.Taxonomy = tu.Taxonomy[:len(tu.Taxonomy)-1] },
		"concepts range inverted":       func(tu *Tuning) { tu.Write.MinConcepts = tu.Write.MaxConcepts + 1 },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			tu := DefaultTuning()
			breakIt(&tu)
			if err := tu.Validate(); err == nil {
				t.Error("want an error")
			}
		})
	}
}
