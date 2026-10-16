package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alternayte/drel"

	"github.com/alternayte/briefing/internal/platform"
)

func TestPipelineRunsFromPollToStory(t *testing.T) {
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

	// The model stub keeps every item and gives each text its own direction,
	// so each item opens its own story.
	models := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/embeddings" {
			var req struct {
				Input []string `json:"input"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			type item struct {
				Index     int       `json:"index"`
				Embedding []float32 `json:"embedding"`
			}
			out := struct {
				Data []item `json:"data"`
			}{}
			for i, text := range req.Input {
				v := make([]float32, 1536)
				v[len(text)%1536] = 1
				v[(i*97+7)%1536] += 0.5
				out.Data = append(out.Data, item{Index: i, Embedding: v})
			}
			_ = json.NewEncoder(w).Encode(out)
			return
		}
		fmt.Fprint(w, `{"answers":{"worth":{"type":"noul","noul":0.9},"sensational":{"type":"noul","noul":0.1},
			"category":{"type":"choice","choice":"software_engineering"},"significance":{"type":"score","score":2.5},
			"learning":{"type":"score","score":2},"same_event":{"type":"noul","noul":0.1}},"usage":{"input_tokens":1,"output_tokens":0,"cost":0}}`)
	}))
	defer models.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := platform.OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	engine, err := drel.NewEngine(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(engine.Close)

	var sourceID int64
	url := feed.URL + "/front_page?job-test=" + time.Now().Format(time.RFC3339Nano)
	if err := pool.QueryRow(ctx, `INSERT INTO sources (kind, url, title, tier, language, poll_interval_seconds)
		VALUES ('hn', $1, 'job test', 2, 'en', 600) RETURNING id`, url).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM stories WHERE id IN (SELECT si.story_id FROM story_items si JOIN sightings s ON s.item_id = si.item_id WHERE s.source_id = $1)`, sourceID)
		_, _ = pool.Exec(ctx, `DELETE FROM items WHERE id IN (SELECT item_id FROM sightings WHERE source_id = $1)`, sourceID)
		_, _ = pool.Exec(ctx, `DELETE FROM sources WHERE id = $1`, sourceID)
	})

	// Fixed so the fixture's items, dated 2026-10-16, pass the age rule.
	now := time.Date(2026, 10, 16, 3, 30, 0, 0, time.UTC)
	cfg := platform.Config{DecideAPI: platform.DecideAPIDecisions, DecideModel: "cloudflare/clef", EmbedModel: "test-embed", EmbedDim: 1536}
	p := pipeline{
		cfg:        cfg,
		engine:     engine,
		openrouter: platform.NewOpenRouter(models.URL, "k", models.Client(), engine, func() time.Time { return now }),
		tuning:     platform.DefaultTuning(),
		feeds:      feed.Client(),
		pages:      feed.Client(),
		userAgent:  "test",
		now:        func() time.Time { return now },
	}
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- runWorkers(runCtx, pool, p) }()

	deadline := time.Now().Add(20 * time.Second)
	var clustered int
	for time.Now().Before(deadline) {
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM sightings s JOIN story_items si ON si.item_id = s.item_id
			WHERE s.source_id = $1`, sourceID).Scan(&clustered)
		if clustered == 3 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if clustered != 3 {
		rows, _ := pool.Query(ctx, `SELECT kind, state, COALESCE(errors::text, '') FROM river_job ORDER BY id DESC LIMIT 10`)
		for rows.Next() {
			var kind, state, errs string
			_ = rows.Scan(&kind, &state, &errs)
			t.Logf("job %s %s %.200s", kind, state, errs)
		}
		rows.Close()
		rows, _ = pool.Query(ctx, `SELECT i.status::text, i.embed_model FROM items i JOIN sightings s ON s.item_id = i.id WHERE s.source_id = $1`, sourceID)
		for rows.Next() {
			var st, em string
			_ = rows.Scan(&st, &em)
			t.Logf("item %s embed=%q", st, em)
		}
		rows.Close()
	}
	stop()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if clustered != 3 {
		t.Fatalf("%d items reached a story, want 3", clustered)
	}
}
