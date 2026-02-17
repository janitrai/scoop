package app

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"horse.fit/scoop/internal/cli"
)

func runSearch(args []string) int {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	envLoader := cli.AddEnvFlag(fs, ".env", "Path to the .env file")
	timeout := fs.Duration("timeout", 30*time.Second, "Command timeout")
	query := fs.String("query", "", "Query text for story title search")
	collection := fs.String("collection", "", "Optional collection filter")
	limit := fs.Int("limit", 20, "Maximum stories to return")
	format := fs.String("format", outputFormatTable, "Output format: table or json")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "search does not accept positional arguments")
		return 2
	}

	trimmedQuery := strings.TrimSpace(*query)
	if trimmedQuery == "" {
		fmt.Fprintln(os.Stderr, "--query is required")
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

	ctx, cancel, pool, err := connectReadPool(*timeout, envLoader)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer cancel()
	defer pool.Close()

	stories, err := pool.SearchStoriesByTitle(ctx, trimmedQuery, normalizeCollectionFlag(*collection), *limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to search stories: %v\n", err)
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
