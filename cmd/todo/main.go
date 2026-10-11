// Command todo runs the todo HTTP API.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"example.com/todo/internal/config"
	"example.com/todo/internal/database"
	"example.com/todo/internal/server"
	"example.com/todo/internal/todo"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(log); err != nil {
		log.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := database.Migrate(cfg.DatabaseURL); err != nil {
		return fmt.Errorf("migrating: %w", err)
	}

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	srv := server.New(cfg.Addr(), log, handlers(pool, log))
	return srv.Run(ctx)
}

func handlers(pool *pgxpool.Pool, log *slog.Logger) []server.Handler {
	todoRepo := todo.NewPostgresRepo(pool)
	todoHandler := todo.NewHandler(todoRepo, log)

	return []server.Handler{
		todoHandler,
	}
}
