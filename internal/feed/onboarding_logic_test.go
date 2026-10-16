package feed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/embed"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

func TestParseTopics(t *testing.T) {
	got := parseTopics("  Rust  and   systems \n\nexoplanets\nrust and systems\n")
	if want := []string{"Rust and systems", "exoplanets"}; !slices.Equal(got, want) {
		t.Errorf("topics = %q", got)
	}
}

// axisEmbedder embeds the n-th text of a request along axis n.
func axisEmbedder(t *testing.T) *embed.Embedder {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Input []string }
		_ = json.NewDecoder(r.Body).Decode(&req)
		type item struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		}
		var data []item
		for i := range req.Input {
			v := make([]float32, 3)
			v[i%3] = 1
			data = append(data, item{i, v})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	t.Cleanup(srv.Close)
	or := platform.NewOpenRouter(srv.URL, "k", srv.Client(), nil, time.Now)
	return embed.New(or, platform.Config{EmbedModel: "test", EmbedDim: 3})
}

func TestCalibrationRatingsMoveTheNearestInterest(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	em := axisEmbedder(t)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		user := pgtest.User(t, ctx, "calibrate@example.com")
		if err := replaceInterests(ctx, e, em, user, []string{"Rust", "biology"}, []string{"crypto"}); err != nil {
			t.Fatal(err)
		}
		pos, neg, err := interestsOf(ctx, tx, user)
		if err != nil || pos != "Rust\nbiology" || neg != "crypto" {
			t.Fatalf("interests = %q / %q (%v)", pos, neg, err)
		}
		story := func(centroid string) int64 {
			var id int64
			_ = tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category, centroid) VALUES (now(), now(), 'x', $1::vector) RETURNING id`, centroid).Scan(&id)
			return id
		}
		rustStory, bioStory := story("[1,0,0]"), story("[0,1,0]")

		if err := rate(ctx, e, user, rustStory, "useful", time.Now(), platform.DefaultTuning().Review); err != nil {
			t.Fatal(err)
		}
		if err := rate(ctx, e, user, rustStory, "irrelevant", time.Now(), platform.DefaultTuning().Review); err != nil {
			t.Fatal(err)
		}
		if err := rate(ctx, e, user, bioStory, "irrelevant", time.Now(), platform.DefaultTuning().Review); err != nil {
			t.Fatal(err)
		}
		weight := func(text string) float64 {
			var w float64
			_ = tx.QueryRow(ctx, `SELECT weight FROM interests WHERE user_id = $1 AND text = $2`, user, text).Scan(&w)
			return w
		}
		if w := weight("Rust"); w < 1.09 || w > 1.11 {
			t.Errorf("Rust weight %v: a second rating of the same story must not count", w)
		}
		if w := weight("biology"); w < 0.89 || w > 0.91 {
			t.Errorf("biology weight %v", w)
		}
		var events int
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM learning_events WHERE user_id = $1`, user).Scan(&events)
		// Each kind is recorded once per story; the weight moved only once.
		if events != 3 {
			t.Errorf("%d events, want 3", events)
		}
	})
}
