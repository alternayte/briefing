package platform_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

func TestOpenRouterRecordsUsagePerDayStageAndModel(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"usage":{"input_tokens":600,"output_tokens":0,"cost":0.00002}}`))
	}))
	defer srv.Close()
	day := time.Date(2031, 1, 2, 10, 0, 0, 0, time.UTC)
	or := platform.NewOpenRouter(srv.URL, "sk-test", srv.Client(), e, func() time.Time { return day })
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		var out struct{ OK bool }
		for range 2 {
			if err := or.Post(ctx, "/api/alpha/decisions", "triage", "cloudflare/clef", map[string]any{}, &out); err != nil {
				t.Fatal(err)
			}
		}
		if !out.OK {
			t.Error("body not decoded")
		}
		var calls, in int
		var cost float64
		err := drel.MustFromContext(ctx).QueryRow(ctx, `SELECT calls, input_tokens, cost_usd::float8 FROM llm_usage
			WHERE day = '2031-01-02' AND provider = 'openrouter' AND stage = 'triage' AND model = 'cloudflare/clef'`).Scan(&calls, &in, &cost)
		if err != nil {
			t.Fatal(err)
		}
		if calls != 2 || in != 1200 || cost < 0.0000399 || cost > 0.0000401 {
			t.Errorf("usage = %d calls, %d tokens, %v", calls, in, cost)
		}
		spent, err := or.SpentOn(ctx, day)
		if err != nil || spent < 0.0000399 {
			t.Errorf("spent = %v (%v)", spent, err)
		}
	})
}

func TestOpenRouterErrorsAreTyped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": 429, "message": "slow down"}})
	}))
	defer srv.Close()
	or := platform.NewOpenRouter(srv.URL, "k", srv.Client(), nil, time.Now)
	err := or.Post(context.Background(), "/x", "triage", "m", map[string]any{}, &struct{}{})
	var apiErr *platform.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 429 || apiErr.Message != "slow down" || !apiErr.Retryable() {
		t.Errorf("err = %#v", err)
	}
}
