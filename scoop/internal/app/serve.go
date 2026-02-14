package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"horse.fit/scoop/internal/cli"
	"horse.fit/scoop/internal/config"
	"horse.fit/scoop/internal/db"
	"horse.fit/scoop/internal/httpapi"
	"horse.fit/scoop/internal/logging"
)

func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	envLoader := cli.AddEnvFlag(fs, ".env", "Path to the .env file")
	host := fs.String("host", "0.0.0.0", "Host interface to bind")
	port := fs.Int("port", 8090, "HTTP port")
	readTimeout := fs.Duration("read-timeout", 10*time.Second, "HTTP read timeout")
	writeTimeout := fs.Duration("write-timeout", 30*time.Second, "HTTP write timeout")
	shutdownTimeout := fs.Duration("shutdown-timeout", 10*time.Second, "Graceful shutdown timeout")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if *port <= 0 || *port > 65535 {
		fmt.Fprintln(os.Stderr, "--port must be between 1 and 65535")
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

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dbCancel()

	pool, err := db.NewPool(dbCtx, cfg)
	if err != nil {
		logger.Error().Err(err).Msg("serve failed to connect to database")
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		return 1
	}
	defer pool.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	go func() {
		<-sigCh
		cancel()
	}()

	srv := httpapi.NewServer(pool, logger, httpapi.Options{
		Host:            *host,
		Port:            *port,
		ReadTimeout:     *readTimeout,
		WriteTimeout:    *writeTimeout,
		ShutdownTimeout: *shutdownTimeout,
	})

	if err := srv.Start(ctx); err != nil {
		logger.Error().Err(err).Str("host", *host).Int("port", *port).Msg("server failed")
		fmt.Fprintf(os.Stderr, "Server failed: %v\n", err)
		return 1
	}

	return 0
}
