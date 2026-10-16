package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/alternayte/drel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/alternayte/briefing/internal/decide"
	"github.com/alternayte/briefing/internal/embed"
	"github.com/alternayte/briefing/internal/ingest"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/stories"
)

// pipeline holds what the job graph needs.
type pipeline struct {
	cfg        platform.Config
	engine     *drel.Engine
	openrouter *platform.OpenRouter
	tuning     platform.Tuning
	feeds      *http.Client
	userAgent  string
	blueskyAPI string
	now        func() time.Time
}

// workers registers every job of the pipeline. Each step enqueues the next one
// after its own transaction commits.
func (p pipeline) workers() (*river.Workers, []*river.PeriodicJob, error) {
	filter, err := stories.NewFilter(p.engine, p.tuning, p.now)
	if err != nil {
		return nil, nil, err
	}
	conn := ingest.NewConnectors(p.feeds, p.tuning, p.userAgent, p.blueskyAPI)
	decider := decide.New(p.openrouter, p.cfg, p.tuning)
	clusterer := stories.NewClusterer(p.engine, decider, p.tuning, p.now)
	w := river.NewWorkers()
	river.AddWorker(w, &ingest.SweepWorker{Engine: p.engine, Now: p.now})
	river.AddWorker(w, &ingest.PollWorker{
		Poller:  ingest.NewPoller(p.engine, conn, p.now),
		OnItems: enqueue(func(ids []int64) river.JobArgs { return stories.FilterArgs{ItemIDs: ids} }),
	})
	river.AddWorker(w, &stories.FilterWorker{Filter: filter, OnPassed: enqueue(func(ids []int64) river.JobArgs { return stories.TriageArgs{ItemIDs: ids} })})
	river.AddWorker(w, &stories.TriageWorker{
		Triager: stories.NewTriager(p.engine, decider, p.tuning, p.now),
		OnKept:  enqueue(func(ids []int64) river.JobArgs { return stories.EmbedArgs{ItemIDs: ids} }),
	})
	river.AddWorker(w, &stories.EmbedWorker{
		Embedder:   stories.NewItemEmbedder(p.engine, embed.New(p.openrouter, p.cfg)),
		OnEmbedded: enqueue(func(ids []int64) river.JobArgs { return stories.ClusterArgs{ItemIDs: ids} }),
	})
	river.AddWorker(w, &stories.ClusterWorker{Clusterer: clusterer})
	river.AddWorker(w, &stories.CloseStaleWorker{Clusterer: clusterer})
	return w, []*river.PeriodicJob{ingest.SweepEveryMinute(), stories.CloseStaleHourly()}, nil
}

// enqueue returns a step hook that inserts the next job for the given ids.
func enqueue(next func(ids []int64) river.JobArgs) func(context.Context, []int64) error {
	return func(ctx context.Context, ids []int64) error {
		_, err := river.ClientFromContext[pgx.Tx](ctx).Insert(ctx, next(ids), nil)
		return err
	}
}

func userAgent(cfg platform.Config) string {
	return "Briefing/0.1 (+" + cfg.BaseURL.String() + ")"
}

// work runs the job workers until ctx ends.
func work(ctx context.Context, cfg platform.Config) error {
	pool, err := platform.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	engine, err := drel.NewEngine(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open drel engine: %w", err)
	}
	defer engine.Close()
	t := platform.DefaultTuning()
	p := pipeline{
		cfg:        cfg,
		openrouter: platform.NewOpenRouter(platform.OpenRouterBase, cfg.OpenRouterAPIKey, &http.Client{Timeout: 2 * time.Minute}, engine, time.Now),
		engine:     engine,
		tuning:     t,
		feeds:      platform.NewPublicClient(t.Poll.Timeout, t.Fetch.MaxRedirects),
		userAgent:  userAgent(cfg),
		now:        time.Now,
	}
	return runWorkers(ctx, pool, p)
}

func runWorkers(ctx context.Context, pool *pgxpool.Pool, p pipeline) error {
	workers, periodic, err := p.workers()
	if err != nil {
		return err
	}
	client, err := platform.NewRiver(pool, workers, periodic)
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("start workers: %w", err)
	}
	slog.Info("workers started")
	<-ctx.Done()
	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return client.Stop(stopCtx)
}
