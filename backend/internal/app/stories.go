package app

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"horse.fit/scoop/internal/cli"
	"horse.fit/scoop/internal/db"
)

func runStories(args []string) int {
	fs := flag.NewFlagSet("stories", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	envLoader := cli.AddEnvFlag(fs, ".env", "Path to the .env file")
	timeout := fs.Duration("timeout", 30*time.Second, "Command timeout")
	collection := fs.String("collection", "", "Filter by collection")
	from := fs.String("from", defaultUTCDayString(), "Start date in YYYY-MM-DD (UTC)")
	to := fs.String("to", defaultUTCDayString(), "End date in YYYY-MM-DD (UTC)")
	limit := fs.Int("limit", 50, "Maximum stories to return")
	format := fs.String("format", outputFormatTable, "Output format: table or json")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "stories does not accept positional arguments")
		return 2
	}
	if *limit <= 0 {
		fmt.Fprintln(os.Stderr, "--limit must be > 0")
		return 2
	}

	outputFormat, err := parseOutputFormat(*format, outputFormatTable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid format: %v\n", err)
		return 2
	}

	fromStart, toEnd, err := parseUTCDateRange(*from, *to)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid date range: %v\n", err)
		return 2
	}

	ctx, cancel, pool, err := connectReadPool(*timeout, envLoader)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer cancel()
	defer pool.Close()

	stories, err := pool.ListStoriesByDedupEventWindow(ctx, db.StoryEventListOptions{
		Collection: normalizeCollectionFlag(*collection),
		From:       fromStart,
		To:         toEnd,
		Limit:      *limit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query stories: %v\n", err)
		return 1
	}

	if outputFormat == outputFormatJSON {
		if err := printJSON(stories); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to encode JSON: %v\n", err)
			return 1
		}
		return 0
	}

	if err := writeStorySummaryTable(stories); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to render table: %v\n", err)
		return 1
	}
	return 0
}

func writeStorySummaryTable(items []db.StorySummary) error {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		createdAt := item.CreatedAt
		if item.EventCreatedAt != nil {
			createdAt = item.EventCreatedAt.UTC()
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", item.StoryID),
			truncateForTable(item.CanonicalTitle, 80),
			pointerStringOrEmpty(item.SourceDomain),
			fmt.Sprintf("%d", item.ArticleCount),
			formatUTCDate(createdAt),
		})
	}

	return writeTable(
		[]string{"story_id", "canonical_title", "source_domain", "article_count", "created_date"},
		rows,
	)
}
