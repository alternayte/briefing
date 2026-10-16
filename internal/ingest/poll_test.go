package ingest

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/alternayte/drel"
	"github.com/alternayte/drel/dreltest"

	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/platform/pgtest"
	"github.com/alternayte/briefing/internal/sources"
)

func seedSource(t *testing.T, ctx context.Context, e *drel.Engine, kind sources.SourceKind, url string) int64 {
	t.Helper()
	err := sources.Seed(ctx, e, []sources.Entry{{Kind: kind, URL: url, Title: "test", Tier: 1, Language: "en", PollInterval: time.Hour}})
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := drel.MustFromContext(ctx).QueryRow(ctx, `SELECT id FROM sources WHERE kind = $1 AND url = $2`, kind, url).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func count(t *testing.T, ctx context.Context, q string, args ...any) int {
	t.Helper()
	var n int
	if err := drel.MustFromContext(ctx).QueryRow(ctx, q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRepollCreatesNoDuplicates(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	srv := serveFixture(t, "testdata/hn.json", "application/json", nil)
	p := NewPoller(e, testConnectors(srv), time.Now)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		src := seedSource(t, ctx, e, sources.KindHN, srv.URL+"/hn?poll-test=dup")
		first, err := p.Poll(ctx, src)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.Poll(ctx, src); err != nil {
			t.Fatal(err)
		}
		items := count(t, ctx, `SELECT count(DISTINCT item_id) FROM sightings WHERE source_id = $1`, src)
		sightings := count(t, ctx, `SELECT count(*) FROM sightings WHERE source_id = $1`, src)
		if len(first) != 3 || items != 3 || sightings != 3 {
			t.Errorf("first poll %d ids; after two polls %d items, %d sightings", len(first), items, sightings)
		}
		if n := count(t, ctx, `SELECT count(*) FROM sources WHERE id = $1 AND last_ok_at IS NOT NULL AND last_error = ''`, src); n != 1 {
			t.Error("checkpoint not saved")
		}
	})
}

func TestOneLinkSeenByTwoSourcesIsOneItem(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	srv := serveFixture(t, "testdata/hn.json", "application/json", nil)
	p := NewPoller(e, testConnectors(srv), time.Now)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		a := seedSource(t, ctx, e, sources.KindHN, srv.URL+"/hn?poll-test=a")
		b := seedSource(t, ctx, e, sources.KindHN, srv.URL+"/hn?poll-test=b")
		ia, _ := p.Poll(ctx, a)
		ib, _ := p.Poll(ctx, b)
		slices.Sort(ia)
		slices.Sort(ib)
		if len(ia) != 3 || !slices.Equal(ia, ib) {
			t.Errorf("ids differ: %v vs %v", ia, ib)
		}
		if n := count(t, ctx, `SELECT count(*) FROM sightings WHERE source_id IN ($1, $2)`, a, b); n != 6 {
			t.Errorf("%d sightings, want 6", n)
		}
	})
}

func TestFailingSourceRecordsItsError(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	srv := serveFixture(t, "testdata/krebs.rss", "application/rss+xml", func(w http.ResponseWriter, r *http.Request) bool {
		http.Error(w, "down", http.StatusServiceUnavailable)
		return true
	})
	p := NewPoller(e, testConnectors(srv), time.Now)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		src := seedSource(t, ctx, e, sources.KindRSS, srv.URL+"/feed?poll-test=fail")
		if _, err := p.Poll(ctx, src); err == nil {
			t.Fatal("want an error")
		}
		if n := count(t, ctx, `SELECT count(*) FROM sources WHERE id = $1 AND last_error LIKE '%503%'`, src); n != 1 {
			t.Error("last_error not recorded")
		}
	})
}

func TestDisabledSourceIsNotFetched(t *testing.T) {
	t.Parallel()
	e := pgtest.Engine(t)
	var hits int
	srv := serveFixture(t, "testdata/hn.json", "application/json", func(w http.ResponseWriter, r *http.Request) bool { hits++; return false })
	p := NewPoller(e, NewConnectors(srv.Client(), platform.DefaultTuning(), "t", srv.URL), time.Now)
	dreltest.WithRollback(t, e, func(ctx context.Context) {
		src := seedSource(t, ctx, e, sources.KindHN, srv.URL+"/hn?poll-test=off")
		if _, err := drel.MustFromContext(ctx).Exec(ctx, `UPDATE sources SET enabled = false WHERE id = $1`, src); err != nil {
			t.Fatal(err)
		}
		if ids, err := p.Poll(ctx, src); err != nil || len(ids) != 0 || hits != 0 {
			t.Errorf("disabled source polled: ids=%v err=%v hits=%d", ids, err, hits)
		}
	})
}
