package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alternayte/drel"

	"github.com/alternayte/briefing/internal/platform"
)

func TestSweepPollAndFilterRunAsJobs(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	body, err := os.ReadFile("../../internal/ingest/testdata/hn.json")
	if err != nil {
		t.Fatal(err)
	}
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer feed.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := platform.OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	engine, err := drel.NewEngine(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	var sourceID int64
	url := feed.URL + "/front_page?job-test=" + time.Now().Format(time.RFC3339Nano)
	if err := pool.QueryRow(ctx, `INSERT INTO sources (kind, url, title, tier, language, poll_interval_seconds)
		VALUES ('hn', $1, 'job test', 2, 'en', 600) RETURNING id`, url).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM items WHERE id IN (SELECT item_id FROM sightings WHERE source_id = $1)`, sourceID)
		_, _ = pool.Exec(ctx, `DELETE FROM sources WHERE id = $1`, sourceID)
	})

	// Fixed so the fixture's items, dated 2026-10-16, pass the age rule.
	now := time.Date(2026, 10, 16, 3, 30, 0, 0, time.UTC)
	p := pipeline{engine: engine, tuning: platform.DefaultTuning(), feeds: feed.Client(), userAgent: "test", now: func() time.Time { return now }}
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- runWorkers(runCtx, pool, p) }()

	deadline := time.Now().Add(20 * time.Second)
	var passed int
	for time.Now().Before(deadline) {
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM items i JOIN sightings s ON s.item_id = i.id
			WHERE s.source_id = $1 AND i.status = 'passed'`, sourceID).Scan(&passed)
		if passed == 3 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	stop()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if passed != 3 {
		t.Fatalf("%d items passed the filter, want 3", passed)
	}
}
