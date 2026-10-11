// Command todo runs the todo HTTP API.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/todo/internal/config"
	"example.com/todo/internal/database"
	"example.com/todo/internal/server"
	"example.com/todo/internal/telemetry"
	"example.com/todo/internal/todo"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/bridges/otelslog"
)

func main() {
	stdout := slog.NewJSONHandler(os.Stdout, nil)

	if err := run(stdout); err != nil {
		slog.New(stdout).Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(stdout slog.Handler) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	shutdownTelemetry, err := telemetry.Setup(ctx, cfg)
	if err != nil {
		return fmt.Errorf("setting up telemetry: %w", err)
	}

	defer func() {
		// ctx already is cancelled with SIGTERM, so flush with a fresh one
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := shutdownTelemetry(flushCtx); err != nil {
			fmt.Fprintln(os.Stderr, "telemetry shutdown:", err)
		}
	}()

	logHandlers := []slog.Handler{stdout}

	if cfg.OTLPEndpoint != "" {
		logHandlers = append(logHandlers, otelslog.NewHandler(cfg.OTELServiceName))
	}

	log := slog.New(slog.NewMultiHandler(logHandlers...))

	if err := database.Migrate(cfg.DatabaseURL); err != nil {
		return fmt.Errorf("migrating: %w", err)
	}

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	httpHandlers, err := handlers(pool, log)
	if err != nil {
		return err
	}

	srv := server.New(cfg.Addr(), log, httpHandlers)
	return srv.Run(ctx)
}

func handlers(pool *pgxpool.Pool, log *slog.Logger) ([]server.Handler, error) {
	todoRepo := todo.NewPostgresRepo(pool)
	todoHandler, err := todo.NewHandler(todoRepo, log)
	if err != nil {
		return nil, err
	}

	return []server.Handler{
		todoHandler,
	}, nil
}
