package app

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"horse.fit/scoop/internal/cli"
)

func runStats(args []string) int {
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	envLoader := cli.AddEnvFlag(fs, ".env", "Path to the .env file")
	timeout := fs.Duration("timeout", 30*time.Second, "Command timeout")
	format := fs.String("format", outputFormatTable, "Output format: table or json")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "stats does not accept positional arguments")
		return 2
	}

	outputFormat, err := parseOutputFormat(*format, outputFormatTable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid format: %v\n", err)
		return 2
	}

	ctx, cancel, pool, err := connectReadPool(*timeout, envLoader)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer cancel()
	defer pool.Close()

	dayStart := defaultUTCDay()
	_, dayEnd := utcDayBounds(dayStart)

	stats, err := pool.QueryPipelineStats(ctx, dayStart, dayEnd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query pipeline stats: %v\n", err)
		return 1
	}

	if outputFormat == outputFormatJSON {
		if err := printJSON(stats); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to encode JSON: %v\n", err)
			return 1
		}
		return 0
	}

	collectionRows := make([][]string, 0, len(stats.Collections)+1)
	for _, row := range stats.Collections {
		collectionRows = append(collectionRows, []string{
			row.Collection,
			fmt.Sprintf("%d", row.Articles),
			fmt.Sprintf("%d", row.Stories),
			fmt.Sprintf("%d", row.Embeddings),
		})
	}
	collectionRows = append(collectionRows, []string{
		"TOTAL",
		fmt.Sprintf("%d", stats.Totals.Articles),
		fmt.Sprintf("%d", stats.Totals.Stories),
		fmt.Sprintf("%d", stats.Totals.Embeddings),
	})

	if err := writeTable([]string{"collection", "articles", "stories", "embeddings"}, collectionRows); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to render collection table: %v\n", err)
		return 1
	}

	fmt.Println()
	throughputRows := [][]string{
		{"articles_ingested_today", fmt.Sprintf("%d", stats.Throughput.ArticlesIngestedToday)},
		{"stories_created_today", fmt.Sprintf("%d", stats.Throughput.StoriesCreatedToday)},
		{"pending_not_embedded", fmt.Sprintf("%d", stats.Throughput.PendingNotEmbedded)},
		{"pending_not_deduped", fmt.Sprintf("%d", stats.Throughput.PendingNotDeduped)},
	}
	if err := writeTable([]string{"metric", "value"}, throughputRows); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to render throughput table: %v\n", err)
		return 1
	}

	return 0
}
