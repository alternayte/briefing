// Command briefing runs the Briefing web server and its workers.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alternayte/drel"

	"github.com/alternayte/briefing/internal/accounts"
	"github.com/alternayte/briefing/internal/admin"
	"github.com/alternayte/briefing/internal/embed"
	"github.com/alternayte/briefing/internal/feed"
	"github.com/alternayte/briefing/internal/platform"
	"github.com/alternayte/briefing/internal/write"
)

// explainsPerDay limits each reader's explanations, which cost a writer call.
const explainsPerDay = 20

const usage = "usage: briefing all|serve|work|migrate|seed"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "briefing:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	// gx dev starts the binary with no arguments and gives it the address.
	devAddr := os.Getenv("GX_DEV_ADDR")
	if len(args) == 0 && devAddr != "" {
		args = []string{"serve"}
	}
	if len(args) != 1 {
		return errors.New(usage)
	}
	cfg, err := platform.LoadConfigFromEnv()
	if err != nil {
		return err
	}
	if devAddr != "" {
		cfg.Addr = devAddr
	}
	if err := platform.DefaultTuning().Validate(); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch args[0] {
	case "serve":
		return serve(ctx, cfg)
	case "migrate":
		return migrate(ctx, cfg)
	case "seed":
		return seed(ctx, cfg)
	case "work":
		return work(ctx, cfg)
	case "all":
		return all(ctx, cfg)
	default:
		return fmt.Errorf("unknown command %q; %s", args[0], usage)
	}
}

func serve(ctx context.Context, cfg platform.Config) error {
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
	auth, err := accounts.NewAuth(cfg, pool)
	if err != nil {
		return err
	}
	if err := auth.CheckSchema(ctx); err != nil {
		return fmt.Errorf("auth schema: %w; run briefing migrate", err)
	}
	tuning := platform.DefaultTuning()
	profiles := accounts.NewProfileStore(engine, tuning)
	web := accounts.NewWeb(cfg, auth, profiles)
	jobs, err := platform.NewRiver(pool, nil, nil)
	if err != nil {
		return err
	}
	or := platform.NewOpenRouter(platform.OpenRouterBase, cfg.OpenRouterAPIKey, &http.Client{Timeout: time.Minute}, engine, time.Now)
	feedWeb := feed.NewWeb(engine, embed.New(or, cfg), write.NewExplainer(engine, or, cfg, explainsPerDay, time.Now), profiles, jobs, tuning, time.Now)

	adminWeb := admin.NewWeb(engine, cfg, time.Now)
	srv := &http.Server{Addr: cfg.Addr, Handler: newHandler(web, feedWeb, adminWeb, pool.Ping), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.Addr)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

// all migrates, loads the catalog, then runs the server and the workers in
// one process. When one stops, the other stops too.
func all(ctx context.Context, cfg platform.Config) error {
	if err := migrate(ctx, cfg); err != nil {
		return err
	}
	if err := seed(ctx, cfg); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errc := make(chan error, 2)
	go func() { errc <- serve(ctx, cfg) }()
	go func() { errc <- work(ctx, cfg) }()
	first := <-errc
	cancel()
	if second := <-errc; first == nil {
		first = second
	}
	return first
}
