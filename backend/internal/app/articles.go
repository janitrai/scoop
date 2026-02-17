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

func runArticles(args []string) int {
	fs := flag.NewFlagSet("articles", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	envLoader := cli.AddEnvFlag(fs, ".env", "Path to the .env file")
	timeout := fs.Duration("timeout", 30*time.Second, "Command timeout")
	collection := fs.String("collection", "", "Filter by collection")
	from := fs.String("from", defaultUTCDayString(), "Start date in YYYY-MM-DD (UTC)")
	to := fs.String("to", defaultUTCDayString(), "End date in YYYY-MM-DD (UTC)")
	limit := fs.Int("limit", 50, "Maximum articles to return")
	format := fs.String("format", outputFormatTable, "Output format: table or json")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "articles does not accept positional arguments")
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

	articles, err := pool.ListArticles(ctx, db.ArticleListOptions{
		Collection: normalizeCollectionFlag(*collection),
		From:       fromStart,
		To:         toEnd,
		Limit:      *limit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query articles: %v\n", err)
		return 1
	}

	if outputFormat == outputFormatJSON {
		if err := printJSON(articles); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to encode JSON: %v\n", err)
			return 1
		}
		return 0
	}

	tableRows := make([][]string, 0, len(articles))
	for _, article := range articles {
		tableRows = append(tableRows, []string{
			fmt.Sprintf("%d", article.ArticleID),
			truncateForTable(article.Title, 80),
			article.Source,
			pointerStringOrEmpty(article.SourceDomain),
			formatUTCTimestampPtr(article.PublishedAt),
			article.Collection,
			formatUTCTimestamp(article.CreatedAt),
		})
	}

	if err := writeTable(
		[]string{"article_id", "title", "source", "source_domain", "published_at", "collection", "created_at"},
		tableRows,
	); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to render table: %v\n", err)
		return 1
	}

	return 0
}
