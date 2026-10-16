package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthzReportsDatabaseState(t *testing.T) {
	for _, c := range []struct {
		ping func(context.Context) error
		want int
	}{
		{func(context.Context) error { return nil }, http.StatusOK},
		{func(context.Context) error { return errors.New("down") }, http.StatusServiceUnavailable},
	} {
		rec := httptest.NewRecorder()
		healthz(c.ping).ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
		if rec.Code != c.want {
			t.Errorf("got %d, want %d", rec.Code, c.want)
		}
	}
}

func TestLLMsTxtNamesTheMCPEndpointAndTheAuth(t *testing.T) {
	rec := httptest.NewRecorder()
	llmsTxt("https://briefing.example.com/").ServeHTTP(rec, httptest.NewRequest("GET", "/llms.txt", nil))
	body := rec.Body.String()
	for _, want := range []string{"https://briefing.example.com/mcp", "Bearer brf_", "feed_today", "skills/briefing"} {
		if !strings.Contains(body, want) {
			t.Errorf("llms.txt lacks %q", want)
		}
	}
}
