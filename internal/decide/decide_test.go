package decide

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/alternayte/briefing/internal/platform"
)

type captured struct {
	path string
	body map[string]any
}

func stub(t *testing.T, fixture string) (*httptest.Server, *captured) {
	t.Helper()
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	c := &captured{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.path = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &c.body)
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv, c
}

func triageQuestions() Questions {
	return Questions{
		"worth":        Noul("Does it report something new?", "It does.", "It does not."),
		"category":     Choice("Which topic?", "programming_languages", "security", "other"),
		"significance": Score("How significant?", "trivial", "minor", "notable", "major", "landmark"),
	}
}

func TestDecisionsAPIAnswersAreTyped(t *testing.T) {
	srv, c := stub(t, "testdata/clef-triage.json")
	cfg := platform.Config{DecideAPI: platform.DecideAPIDecisions, DecideModel: "cloudflare/clef"}
	d := New(platform.NewOpenRouter(srv.URL, "k", srv.Client(), nil, time.Now), cfg, platform.DefaultTuning())
	ans, err := d.Ask(context.Background(), "triage", "Title: x", triageQuestions())
	if err != nil {
		t.Fatal(err)
	}
	if c.path != "/api/alpha/decisions" || c.body["model"] != "cloudflare/clef" {
		t.Errorf("request = %s %v", c.path, c.body["model"])
	}
	if ans["worth"].Noul < 0.9 || ans["category"].Choice != "programming_languages" {
		t.Errorf("answers = %+v", ans)
	}
	if s := ans["significance"]; s.Score < 2 || s.Score > 3 || s.Probabilities["3"] == 0 {
		t.Errorf("score = %+v", s)
	}
}

func TestSystemOneUsesItsPath(t *testing.T) {
	srv, c := stub(t, "testdata/systemone-triage.json")
	cfg := platform.Config{DecideAPI: platform.DecideAPISystemOne, DecideModel: "~typesafe/jev-latest"}
	d := New(platform.NewOpenRouter(srv.URL, "k", srv.Client(), nil, time.Now), cfg, platform.DefaultTuning())
	ans, err := d.Ask(context.Background(), "triage", "Title: x", triageQuestions())
	if err != nil {
		t.Fatal(err)
	}
	if c.path != "/api/v1/systemone" || ans["worth"].Noul == 0 {
		t.Errorf("path %s answers %+v", c.path, ans)
	}
}

func TestStateNeverExceedsTheCap(t *testing.T) {
	srv, c := stub(t, "testdata/clef-triage.json")
	tu := platform.DefaultTuning()
	d := New(platform.NewOpenRouter(srv.URL, "k", srv.Client(), nil, time.Now), platform.Config{DecideAPI: platform.DecideAPIDecisions}, tu)
	long := strings.Repeat("é", tu.Triage.MaxStateChars*2)
	if _, err := d.Ask(context.Background(), "triage", long, triageQuestions()); err != nil {
		t.Fatal(err)
	}
	if n := utf8.RuneCountInString(c.body["state"].(string)); n > tu.Triage.MaxStateChars {
		t.Errorf("sent %d characters of state", n)
	}
}

func TestMissingAnswerIsAnError(t *testing.T) {
	srv, _ := stub(t, "testdata/clef-triage.json")
	d := New(platform.NewOpenRouter(srv.URL, "k", srv.Client(), nil, time.Now), platform.Config{DecideAPI: platform.DecideAPIDecisions}, platform.DefaultTuning())
	_, err := d.Ask(context.Background(), "triage", "x", Questions{"unasked": Noul("?", "y", "n")})
	if err == nil {
		t.Error("an answer set without the asked question was accepted")
	}
}
