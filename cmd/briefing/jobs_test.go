package main

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/alternayte/briefing/internal/content"
	"github.com/alternayte/briefing/internal/ingest"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/ranking"
	"github.com/alternayte/briefing/internal/stories"
	"github.com/alternayte/briefing/internal/translate"
	"github.com/alternayte/briefing/internal/write"
)

// River checks the insert options of a job only when it is inserted, so a
// bad option shows up in production. This inserts one of every job and rolls
// back.
func TestEveryJobCanBeInserted(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := platform.OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	client, err := platform.NewRiver(pool, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	jobs := []river.JobArgs{
		ingest.PollArgs{SourceID: 1}, ingest.SweepArgs{},
		stories.FilterArgs{}, stories.TriageArgs{}, stories.EmbedArgs{}, stories.ClusterArgs{},
		stories.CloseStaleArgs{}, stories.SweepArgs{},
		content.FetchArgs{StoryID: 1}, write.WriteArgs{StoryID: 1},
		ranking.RankArgs{UserID: uuid.New()}, ranking.RankAllArgs{},
		translate.TranslateArgs{StoryID: 1, Locale: "fr"}, translate.SweepArgs{},
	}
	for _, j := range jobs {
		if _, err := client.InsertTx(ctx, tx, j, nil); err != nil {
			t.Errorf("%s: %v", j.Kind(), err)
		}
	}
}
