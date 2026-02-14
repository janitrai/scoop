package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"horse.fit/scoop/internal/cli"
	"horse.fit/scoop/internal/config"
	"horse.fit/scoop/internal/db"
	"horse.fit/scoop/internal/logging"
	"horse.fit/scoop/internal/pipeline"
)

func runNormalize(args []string) int {
	fs := flag.NewFlagSet("normalize", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	envLoader := cli.AddEnvFlag(fs, ".env", "Path to the .env file")
	timeout := fs.Duration("timeout", 60*time.Second, "Command timeout")
	limit := fs.Int("limit", 1000, "Maximum pending raw arrivals to normalize")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *limit <= 0 {
		fmt.Fprintln(os.Stderr, "--limit must be > 0")
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
		logger.Error().Err(err).Msg("normalize command failed to connect to database")
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		return 1
	}
	defer pool.Close()

	svc := pipeline.NewService(pool, logger)
	result, err := svc.NormalizePending(ctx, *limit)
	if err != nil {
		logger.Error().Err(err).Int("limit", *limit).Msg("normalize failed")
		fmt.Fprintf(os.Stderr, "Normalize failed: %v\n", err)
		return 1
	}

	logger.Info().
		Int("limit", *limit).
		Int("processed", result.Processed).
		Int("inserted", result.Inserted).
		Msg("normalize completed")
	fmt.Printf("normalize processed=%d inserted=%d limit=%d\n", result.Processed, result.Inserted, *limit)
	return 0
}

func runEmbed(args []string) int {
	fs := flag.NewFlagSet("embed", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	envLoader := cli.AddEnvFlag(fs, ".env", "Path to the .env file")
	timeout := fs.Duration("timeout", 2*time.Minute, "Command timeout")
	limit := fs.Int("limit", 1000, "Maximum pending documents to embed")
	batchSize := fs.Int("batch-size", pipeline.DefaultEmbeddingBatchSize, "Embedding request batch size")
	endpoint := fs.String("endpoint", pipeline.DefaultEmbeddingEndpoint, "Embedding HTTP endpoint")
	modelName := fs.String("model-name", pipeline.DefaultEmbeddingModelName, "Embedding model name key for storage")
	modelVersion := fs.String("model-version", pipeline.DefaultEmbeddingModelVersion, "Embedding model version key for storage")
	maxLength := fs.Int("max-length", pipeline.DefaultEmbeddingMaxLength, "Embedding max token length per text")
	requestTimeout := fs.Duration("request-timeout", pipeline.DefaultEmbeddingRequestTimeout, "Per-request timeout for embedding API")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *limit <= 0 {
		fmt.Fprintln(os.Stderr, "--limit must be > 0")
		return 2
	}
	if *batchSize <= 0 {
		fmt.Fprintln(os.Stderr, "--batch-size must be > 0")
		return 2
	}
	if *maxLength <= 0 {
		fmt.Fprintln(os.Stderr, "--max-length must be > 0")
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
		logger.Error().Err(err).Msg("embed command failed to connect to database")
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		return 1
	}
	defer pool.Close()

	svc := pipeline.NewService(pool, logger)
	result, err := svc.EmbedPending(ctx, pipeline.EmbedOptions{
		Limit:          *limit,
		BatchSize:      *batchSize,
		Endpoint:       *endpoint,
		ModelName:      *modelName,
		ModelVersion:   *modelVersion,
		MaxLength:      *maxLength,
		RequestTimeout: *requestTimeout,
	})
	if err != nil {
		logger.Error().Err(err).Int("limit", *limit).Msg("embed failed")
		fmt.Fprintf(os.Stderr, "Embed failed: %v\n", err)
		return 1
	}

	logger.Info().
		Int("limit", *limit).
		Int("processed", result.Processed).
		Int("embedded", result.Embedded).
		Int("skipped", result.Skipped).
		Int("failed", result.Failed).
		Msg("embed completed")
	fmt.Printf(
		"embed processed=%d embedded=%d skipped=%d failed=%d limit=%d model=%s model_version=%s\n",
		result.Processed,
		result.Embedded,
		result.Skipped,
		result.Failed,
		*limit,
		*modelName,
		*modelVersion,
	)
	return 0
}

func runDedup(args []string) int {
	fs := flag.NewFlagSet("dedup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	envLoader := cli.AddEnvFlag(fs, ".env", "Path to the .env file")
	timeout := fs.Duration("timeout", 90*time.Second, "Command timeout")
	limit := fs.Int("limit", 1000, "Maximum pending documents to deduplicate")
	modelName := fs.String("model-name", pipeline.DefaultEmbeddingModelName, "Embedding model name used for semantic dedup")
	modelVersion := fs.String("model-version", pipeline.DefaultEmbeddingModelVersion, "Embedding model version used for semantic dedup")
	lookbackDays := fs.Int("lookback-days", pipeline.DefaultDedupLookbackDays, "How many days of stories to search for lexical/semantic candidates")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *limit <= 0 {
		fmt.Fprintln(os.Stderr, "--limit must be > 0")
		return 2
	}
	if *lookbackDays <= 0 {
		fmt.Fprintln(os.Stderr, "--lookback-days must be > 0")
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
		logger.Error().Err(err).Msg("dedup command failed to connect to database")
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		return 1
	}
	defer pool.Close()

	svc := pipeline.NewService(pool, logger)
	result, err := svc.DedupPending(ctx, pipeline.DedupOptions{
		Limit:        *limit,
		ModelName:    *modelName,
		ModelVersion: *modelVersion,
		LookbackDays: *lookbackDays,
	})
	if err != nil {
		logger.Error().Err(err).Int("limit", *limit).Msg("dedup failed")
		fmt.Fprintf(os.Stderr, "Dedup failed: %v\n", err)
		return 1
	}

	logger.Info().
		Int("limit", *limit).
		Int("lookback_days", *lookbackDays).
		Int("processed", result.Processed).
		Int("new_stories", result.NewStories).
		Int("auto_merges", result.AutoMerges).
		Int("gray_zones", result.GrayZones).
		Msg("dedup completed")
	fmt.Printf(
		"dedup processed=%d new_stories=%d auto_merges=%d gray_zones=%d limit=%d lookback_days=%d model=%s model_version=%s\n",
		result.Processed,
		result.NewStories,
		result.AutoMerges,
		result.GrayZones,
		*limit,
		*lookbackDays,
		*modelName,
		*modelVersion,
	)
	return 0
}

func runProcess(args []string) int {
	fs := flag.NewFlagSet("process", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	envLoader := cli.AddEnvFlag(fs, ".env", "Path to the .env file")
	timeout := fs.Duration("timeout", 5*time.Minute, "Command timeout")
	normalizeLimit := fs.Int("normalize-limit", 1000, "Maximum raw arrivals to normalize per cycle")
	embedLimit := fs.Int("embed-limit", 1000, "Maximum documents to embed per cycle")
	embedBatchSize := fs.Int("embed-batch-size", pipeline.DefaultEmbeddingBatchSize, "Embedding request batch size")
	embedEndpoint := fs.String("embed-endpoint", pipeline.DefaultEmbeddingEndpoint, "Embedding HTTP endpoint")
	modelName := fs.String("model-name", pipeline.DefaultEmbeddingModelName, "Embedding model name")
	modelVersion := fs.String("model-version", pipeline.DefaultEmbeddingModelVersion, "Embedding model version")
	embedMaxLength := fs.Int("embed-max-length", pipeline.DefaultEmbeddingMaxLength, "Embedding max token length per text")
	embedRequestTimeout := fs.Duration("embed-request-timeout", pipeline.DefaultEmbeddingRequestTimeout, "Per-request timeout for embedding API")
	dedupLimit := fs.Int("dedup-limit", 1000, "Maximum documents to deduplicate per cycle")
	dedupLookbackDays := fs.Int("dedup-lookback-days", pipeline.DefaultDedupLookbackDays, "How many days of stories to search for lexical/semantic candidates")
	untilEmpty := fs.Bool("until-empty", true, "Repeat cycles until no work remains")
	maxCycles := fs.Int("max-cycles", 25, "Maximum normalize+embed+dedup cycles when --until-empty=true")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *normalizeLimit <= 0 {
		fmt.Fprintln(os.Stderr, "--normalize-limit must be > 0")
		return 2
	}
	if *embedLimit <= 0 {
		fmt.Fprintln(os.Stderr, "--embed-limit must be > 0")
		return 2
	}
	if *embedBatchSize <= 0 {
		fmt.Fprintln(os.Stderr, "--embed-batch-size must be > 0")
		return 2
	}
	if *embedMaxLength <= 0 {
		fmt.Fprintln(os.Stderr, "--embed-max-length must be > 0")
		return 2
	}
	if *dedupLimit <= 0 {
		fmt.Fprintln(os.Stderr, "--dedup-limit must be > 0")
		return 2
	}
	if *dedupLookbackDays <= 0 {
		fmt.Fprintln(os.Stderr, "--dedup-lookback-days must be > 0")
		return 2
	}
	if *maxCycles <= 0 {
		fmt.Fprintln(os.Stderr, "--max-cycles must be > 0")
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
		logger.Error().Err(err).Msg("process command failed to connect to database")
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		return 1
	}
	defer pool.Close()

	svc := pipeline.NewService(pool, logger)
	totalNormalize := pipeline.NormalizeResult{}
	totalEmbed := pipeline.EmbedResult{}
	totalDedup := pipeline.DedupResult{}
	cyclesRun := 0
	drained := false

	for cycle := 1; cycle <= *maxCycles; cycle++ {
		normalizeResult, err := svc.NormalizePending(ctx, *normalizeLimit)
		if err != nil {
			logger.Error().Err(err).Int("cycle", cycle).Msg("normalize stage failed")
			fmt.Fprintf(os.Stderr, "Process failed during normalize cycle %d: %v\n", cycle, err)
			return 1
		}

		embedResult, err := svc.EmbedPending(ctx, pipeline.EmbedOptions{
			Limit:          *embedLimit,
			BatchSize:      *embedBatchSize,
			Endpoint:       *embedEndpoint,
			ModelName:      *modelName,
			ModelVersion:   *modelVersion,
			MaxLength:      *embedMaxLength,
			RequestTimeout: *embedRequestTimeout,
		})
		if err != nil {
			logger.Error().Err(err).Int("cycle", cycle).Msg("embed stage failed")
			fmt.Fprintf(os.Stderr, "Process failed during embed cycle %d: %v\n", cycle, err)
			return 1
		}

		dedupResult, err := svc.DedupPending(ctx, pipeline.DedupOptions{
			Limit:        *dedupLimit,
			ModelName:    *modelName,
			ModelVersion: *modelVersion,
			LookbackDays: *dedupLookbackDays,
		})
		if err != nil {
			logger.Error().Err(err).Int("cycle", cycle).Msg("dedup stage failed")
			fmt.Fprintf(os.Stderr, "Process failed during dedup cycle %d: %v\n", cycle, err)
			return 1
		}

		cyclesRun = cycle
		totalNormalize.Processed += normalizeResult.Processed
		totalNormalize.Inserted += normalizeResult.Inserted
		totalEmbed.Processed += embedResult.Processed
		totalEmbed.Embedded += embedResult.Embedded
		totalEmbed.Skipped += embedResult.Skipped
		totalEmbed.Failed += embedResult.Failed
		totalDedup.Processed += dedupResult.Processed
		totalDedup.NewStories += dedupResult.NewStories
		totalDedup.AutoMerges += dedupResult.AutoMerges
		totalDedup.GrayZones += dedupResult.GrayZones

		fmt.Printf(
			"cycle=%d normalize_processed=%d normalize_inserted=%d embed_processed=%d embedded=%d skipped=%d failed=%d dedup_processed=%d new_stories=%d auto_merges=%d gray_zones=%d\n",
			cycle,
			normalizeResult.Processed,
			normalizeResult.Inserted,
			embedResult.Processed,
			embedResult.Embedded,
			embedResult.Skipped,
			embedResult.Failed,
			dedupResult.Processed,
			dedupResult.NewStories,
			dedupResult.AutoMerges,
			dedupResult.GrayZones,
		)

		noProgress := normalizeResult.Processed == 0 && embedResult.Processed == 0 && dedupResult.Processed == 0
		if !*untilEmpty {
			drained = noProgress
			break
		}
		if noProgress {
			drained = true
			break
		}
	}

	logger.Info().
		Int("cycles", cyclesRun).
		Bool("drained", drained).
		Int("normalize_processed", totalNormalize.Processed).
		Int("normalize_inserted", totalNormalize.Inserted).
		Int("embed_processed", totalEmbed.Processed).
		Int("embedded", totalEmbed.Embedded).
		Int("embed_skipped", totalEmbed.Skipped).
		Int("embed_failed", totalEmbed.Failed).
		Int("dedup_processed", totalDedup.Processed).
		Int("new_stories", totalDedup.NewStories).
		Int("auto_merges", totalDedup.AutoMerges).
		Int("gray_zones", totalDedup.GrayZones).
		Msg("process completed")

	fmt.Printf(
		"process_total cycles=%d drained=%t normalize_processed=%d normalize_inserted=%d embed_processed=%d embedded=%d skipped=%d failed=%d dedup_processed=%d new_stories=%d auto_merges=%d gray_zones=%d\n",
		cyclesRun,
		drained,
		totalNormalize.Processed,
		totalNormalize.Inserted,
		totalEmbed.Processed,
		totalEmbed.Embedded,
		totalEmbed.Skipped,
		totalEmbed.Failed,
		totalDedup.Processed,
		totalDedup.NewStories,
		totalDedup.AutoMerges,
		totalDedup.GrayZones,
	)

	if *untilEmpty && !drained {
		fmt.Fprintf(
			os.Stderr,
			"Process stopped after max cycles (%d) before draining queue; rerun with higher --max-cycles or limits\n",
			*maxCycles,
		)
		return 1
	}
	return 0
}
