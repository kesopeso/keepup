// Package database manages PostgreSQL connectivity for the KeepUp API.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"keepup/apps/api/internal/config"
)

const pingRetryInterval = time.Second

// NewPool creates a PostgreSQL connection pool and waits for it to become ready.
func NewPool(ctx context.Context, logger *slog.Logger, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := waitForDatabase(ctx, logger, pool, cfg.StartupTimeout); err != nil {
		pool.Close()
		return nil, fmt.Errorf("wait for postgres: %w", err)
	}

	return pool, nil
}

func waitForDatabase(
	ctx context.Context,
	logger *slog.Logger,
	pool interface {
		Ping(context.Context) error
	},
	timeout time.Duration,
) error {
	startupCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(pingRetryInterval)
	defer ticker.Stop()

	for {
		if err := pool.Ping(startupCtx); err == nil {
			logger.Info("database connection established")
			return nil
		} else {
			logger.Warn("database not ready yet", "error", err)
		}

		select {
		case <-startupCtx.Done():
			return fmt.Errorf("startup timeout reached: %w", startupCtx.Err())
		case <-ticker.C:
		}
	}
}
