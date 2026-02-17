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

type digestOutput struct {
	Date       string            `json:"date"`
	Collection string            `json:"collection"`
	Today      []db.StorySummary `json:"today"`
	Yesterday  []db.StorySummary `json:"yesterday"`
}

func runDigest(args []string) int {
	fs := flag.NewFlagSet("digest", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	envLoader := cli.AddEnvFlag(fs, ".env", "Path to the .env file")
	timeout := fs.Duration("timeout", 30*time.Second, "Command timeout")
	collection := fs.String("collection", "", "Target collection (required)")
	date := fs.String("date", defaultUTCDayString(), "Target date in YYYY-MM-DD (UTC)")
	format := fs.String("format", outputFormatJSON, "Output format: table or json")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "digest does not accept positional arguments")
		return 2
	}

	targetCollection := normalizeCollectionFlag(*collection)
	if targetCollection == "" {
		fmt.Fprintln(os.Stderr, "--collection is required")
		return 2
	}

	outputFormat, err := parseOutputFormat(*format, outputFormatJSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid format: %v\n", err)
		return 2
	}

	targetDay, err := parseUTCDate(*date)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid --date: %v\n", err)
		return 2
	}
	dayStart, dayEnd := utcDayBounds(targetDay)

	yesterday := targetDay.AddDate(0, 0, -1)
	yesterdayStart, yesterdayEnd := utcDayBounds(yesterday)

	ctx, cancel, pool, err := connectReadPool(*timeout, envLoader)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer cancel()
	defer pool.Close()

	todayStories, err := pool.ListDigestStories(ctx, targetCollection, dayStart, dayEnd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query today's digest stories: %v\n", err)
		return 1
	}
	yesterdayStories, err := pool.ListDigestStories(ctx, targetCollection, yesterdayStart, yesterdayEnd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query yesterday's digest stories: %v\n", err)
		return 1
	}

	result := digestOutput{
		Date:       targetDay.Format("2006-01-02"),
		Collection: targetCollection,
		Today:      todayStories,
		Yesterday:  yesterdayStories,
	}

	if outputFormat == outputFormatJSON {
		if err := printJSON(result); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to encode JSON: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Printf("date: %s\n", result.Date)
	fmt.Printf("collection: %s\n\n", result.Collection)

	fmt.Println("today")
	if err := writeStorySummaryTable(result.Today); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to render today table: %v\n", err)
		return 1
	}

	fmt.Println()
	fmt.Println("yesterday")
	if err := writeStorySummaryTable(result.Yesterday); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to render yesterday table: %v\n", err)
		return 1
	}

	return 0
}
