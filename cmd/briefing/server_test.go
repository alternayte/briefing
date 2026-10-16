package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
