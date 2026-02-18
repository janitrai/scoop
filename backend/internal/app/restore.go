package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"horse.fit/scoop/internal/cli"
	"horse.fit/scoop/internal/db"
	"horse.fit/scoop/internal/globaltime"
)

func runRestore(args []string) int {
	if len(args) == 0 {
		printRestoreUsage()
		return 2
	}

	target := strings.ToLower(strings.TrimSpace(args[0]))
	switch target {
	case "story", "article":
	default:
		fmt.Fprintf(os.Stderr, "Unknown restore target: %s\n\n", args[0])
		printRestoreUsage()
		return 2
	}

	fs := flag.NewFlagSet("restore "+target, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	envLoader := cli.AddEnvFlag(fs, ".env", "Path to the .env file")
	timeout := fs.Duration("timeout", 30*time.Second, "Command timeout")
	dryRun := fs.Bool("dry-run", false, "Preview affected rows without applying changes")
	force := fs.Bool("force", false, "Skip confirmation prompt")

	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "restore requires one argument")
		printRestoreUsage()
		return 2
	}

	uuid := strings.TrimSpace(fs.Arg(0))
	if uuid == "" {
		fmt.Fprintln(os.Stderr, "UUID must not be empty")
		return 2
	}

	if !*force {
		ok, err := confirmDangerousAction(fmt.Sprintf("Proceed with restore %s %q?", target, uuid))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read confirmation: %v\n", err)
			return 1
		}
		if !ok {
			fmt.Fprintln(os.Stderr, "Cancelled")
			return 1
		}
	}

	ctx, cancel, pool, err := connectReadPool(*timeout, envLoader)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer cancel()
	defer pool.Close()

	now := globaltime.UTC()
	switch target {
	case "story":
		return runRestoreStory(ctx, pool, uuid, now, *dryRun)
	default:
		return runRestoreArticle(ctx, pool, uuid, now, *dryRun)
	}
}

func runRestoreStory(ctx context.Context, pool *db.Pool, storyUUID string, now time.Time, dryRun bool) int {
	previewCount, err := previewStoryRestoreCount(ctx, pool, storyUUID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to preview story restore: %v\n", err)
		return 1
	}
	if dryRun {
		fmt.Printf("dry_run=true stories_affected=%d\n", previewCount)
		return 0
	}

	affected, err := pool.RestoreStory(ctx, storyUUID, now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to restore story: %v\n", err)
		return 1
	}
	fmt.Printf("stories_affected=%d\n", affected)
	return 0
}

func runRestoreArticle(ctx context.Context, pool *db.Pool, articleUUID string, now time.Time, dryRun bool) int {
	previewCount, err := previewArticleRestoreCount(ctx, pool, articleUUID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to preview article restore: %v\n", err)
		return 1
	}
	if dryRun {
		fmt.Printf("dry_run=true articles_affected=%d\n", previewCount)
		return 0
	}

	affected, err := pool.RestoreArticle(ctx, articleUUID, now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to restore article: %v\n", err)
		return 1
	}
	fmt.Printf("articles_affected=%d\n", affected)
	return 0
}

func previewStoryRestoreCount(ctx context.Context, pool *db.Pool, storyUUID string) (int64, error) {
	const q = `
SELECT COUNT(*)
FROM news.stories
WHERE story_uuid = $1::uuid
  AND deleted_at IS NOT NULL
`
	var count int64
	if err := pool.QueryRow(ctx, q, strings.TrimSpace(storyUUID)).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func previewArticleRestoreCount(ctx context.Context, pool *db.Pool, articleUUID string) (int64, error) {
	const q = `
SELECT COUNT(*)
FROM news.articles
WHERE article_uuid = $1::uuid
  AND deleted_at IS NOT NULL
`
	var count int64
	if err := pool.QueryRow(ctx, q, strings.TrimSpace(articleUUID)).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func printRestoreUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  scoop restore story <story_uuid> [--dry-run] [--force] [--env .env] [--timeout 30s]")
	fmt.Fprintln(os.Stderr, "  scoop restore article <article_uuid> [--dry-run] [--force] [--env .env] [--timeout 30s]")
}
