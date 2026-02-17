package app

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"horse.fit/scoop/internal/cli"
	"horse.fit/scoop/internal/db"
)

func runStoryDetail(args []string) int {
	fs := flag.NewFlagSet("story", flag.ContinueOnError)
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
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "Usage: scoop story <story_uuid> [--format table|json]")
		return 2
	}

	outputFormat, err := parseOutputFormat(*format, outputFormatTable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid format: %v\n", err)
		return 2
	}

	storyUUID := strings.TrimSpace(fs.Arg(0))
	if storyUUID == "" {
		fmt.Fprintln(os.Stderr, "story_uuid is required")
		return 2
	}

	ctx, cancel, pool, err := connectReadPool(*timeout, envLoader)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer cancel()
	defer pool.Close()

	detail, err := pool.GetStoryDetail(ctx, storyUUID)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			fmt.Fprintf(os.Stderr, "Story not found: %s\n", storyUUID)
			return 1
		}
		fmt.Fprintf(os.Stderr, "Failed to load story detail: %v\n", err)
		return 1
	}

	if outputFormat == outputFormatJSON {
		if err := printJSON(detail); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to encode JSON: %v\n", err)
			return 1
		}
		return 0
	}

	if err := writeStoryDetailTable(detail); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to render table: %v\n", err)
		return 1
	}

	return 0
}

func writeStoryDetailTable(detail *db.StoryDetail) error {
	if detail == nil {
		return fmt.Errorf("story detail is nil")
	}

	fmt.Println("story")
	storyRows := [][]string{
		{"story_uuid", detail.Story.StoryUUID},
		{"title", detail.Story.CanonicalTitle},
		{"url", pointerStringOrEmpty(detail.Story.CanonicalURL)},
		{"source_count", fmt.Sprintf("%d", detail.Story.SourceCount)},
		{"article_count", fmt.Sprintf("%d", detail.Story.ArticleCount)},
		{"created_at", formatUTCTimestamp(detail.Story.CreatedAt)},
	}
	if err := writeTable([]string{"field", "value"}, storyRows); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("articles")
	articleRows := make([][]string, 0, len(detail.Articles))
	for _, article := range detail.Articles {
		articleRows = append(articleRows, []string{
			truncateForTable(article.Title, 80),
			pointerStringOrEmpty(article.URL),
			article.Source,
			formatUTCTimestampPtr(article.PublishedAt),
		})
	}
	return writeTable([]string{"title", "url", "source", "published_at"}, articleRows)
}
