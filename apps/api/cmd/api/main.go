package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"keepup/apps/api/internal/config"
	"keepup/apps/api/internal/database"
	"keepup/apps/api/internal/httpapi"
	"keepup/apps/api/internal/routes"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.App.Env)

	dbPool, err := database.NewPool(ctx, logger, cfg.Database)
	if err != nil {
		logger.Error("initialize database", "error", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	routeService := routes.NewService(routes.NewPostgresRepository(dbPool), cfg.Routes.DefaultMaxTrackingMembers)
	handler := httpapi.NewHandler(logger, cfg.App, dbPool, routeService)

	if err := httpapi.Serve(ctx, logger, cfg.App, handler); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("serve api", "error", err)
		os.Exit(1)
	}
}

func newLogger(env string) *slog.Logger {
	level := slog.LevelInfo
	if env == "development" {
		level = slog.LevelDebug
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}
