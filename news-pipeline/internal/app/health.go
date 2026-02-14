package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"horse.fit/news-pipeline/internal/cli"
	"horse.fit/news-pipeline/internal/config"
	"horse.fit/news-pipeline/internal/db"
	"horse.fit/news-pipeline/internal/logging"
)

func runHealth(args []string) int {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	envLoader := cli.AddEnvFlag(fs, ".env", "Path to the .env file")
	timeout := fs.Duration("timeout", 5*time.Second, "Database ping timeout")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if envLoader != nil {
		if _, err := envLoader.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		return 1
	}

	logger, err := logging.New(cfg.Environment, cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		logger.Error().Err(err).Msg("health check failed")
		fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
		return 1
	}
	defer pool.Close()

	logger.Info().
		Dur("timeout", *timeout).
		Msg("database health check passed")
	fmt.Println("ok: database ping successful")
	return 0
}
