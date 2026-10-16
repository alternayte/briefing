package content

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
)

func TestFillStoryStoresTextAndSource(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	srv := articleServer(t)
	short := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><div id="app">Loading…</div></body></html>`))
	}))
	defer short.Close()
	cf := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"result":"# Rendered\n\nThe page as the browser builds it, with the full article text."}`))
	}))
	defer cf.Close()

	tu := platform.DefaultTuning()
	texts := NewTexts(e, NewFetcher(srv.Client(), tu, "test"), NewRenderer(cf.Client(), cf.URL, "a", "t"), tu)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		var story int64
		if err := tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category) VALUES (now(), now(), 'space') RETURNING id`).Scan(&story); err != nil {
			t.Fatal(err)
		}
		add := func(url string) int64 {
			var id int64
			if err := tx.QueryRow(ctx, `INSERT INTO items (canonical_url, title, snippet, status) VALUES ($1, 't', '', 'kept') RETURNING id`, url).Scan(&id); err != nil {
				t.Fatal(err)
			}
			_, _ = tx.Exec(ctx, `INSERT INTO story_items (story_id, item_id) VALUES ($1, $2)`, story, id)
			return id
		}
		article := add(srv.URL + "/story")
		missing := add(srv.URL + "/gone")
		rendered := add(short.URL + "/app")

		n, err := texts.FillStory(ctx, story)
		if err != nil || n != 3 {
			t.Fatalf("filled %d (%v)", n, err)
		}
		check := func(id int64, wantSource string, minLen int) {
			var src string
			var l int
			_ = tx.QueryRow(ctx, `SELECT text_source, text_len FROM items WHERE id = $1`, id).Scan(&src, &l)
			if src != wantSource || l < minLen {
				t.Errorf("item %d: %s %d chars, want %s ≥ %d", id, src, l, wantSource, minLen)
			}
		}
		check(article, "extract", 800)
		check(missing, "render", 10)
		check(rendered, "render", 10)

		if n, _ := texts.FillStory(ctx, story); n != 0 {
			t.Errorf("second fill fetched %d items", n)
		}
	})
}

func TestFillStoryWithoutRendererMarksFailuresNone(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	srv := articleServer(t)
	tu := platform.DefaultTuning()
	texts := NewTexts(e, NewFetcher(srv.Client(), tu, "test"), nil, tu)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		tx := drel.MustFromContext(ctx)
		var story, item int64
		_ = tx.QueryRow(ctx, `INSERT INTO stories (opened_at, last_item_at, category) VALUES (now(), now(), 'space') RETURNING id`).Scan(&story)
		_ = tx.QueryRow(ctx, `INSERT INTO items (canonical_url, title, snippet, status) VALUES ($1, 't', '', 'kept') RETURNING id`, srv.URL+"/gone").Scan(&item)
		_, _ = tx.Exec(ctx, `INSERT INTO story_items (story_id, item_id) VALUES ($1, $2)`, story, item)
		if _, err := texts.FillStory(ctx, story); err != nil {
			t.Fatal(err)
		}
		var src string
		_ = tx.QueryRow(ctx, `SELECT text_source FROM items WHERE id = $1`, item).Scan(&src)
		if src != "none" {
			t.Errorf("source = %s", src)
		}
	})
}
