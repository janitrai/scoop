package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"horse.fit/scoop/internal/cli"
	"horse.fit/scoop/internal/db"
	"horse.fit/scoop/internal/globaltime"
)

type updateStorySnapshot struct {
	StoryUUID  string
	Title      string
	Status     string
	Collection string
	URL        *string
	UpdatedAt  time.Time
}

type updateArticleSnapshot struct {
	ArticleUUID  string
	Title        string
	Source       string
	Collection   string
	URL          *string
	SourceDomain *string
	UpdatedAt    time.Time
}

func runUpdate(args []string) int {
	if len(args) == 0 {
		printUpdateUsage()
		return 2
	}

	target := strings.ToLower(strings.TrimSpace(args[0]))
	switch target {
	case "story", "article":
	default:
		fmt.Fprintf(os.Stderr, "Unknown update target: %s\n\n", args[0])
		printUpdateUsage()
		return 2
	}

	fs := flag.NewFlagSet("update "+target, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	envLoader := cli.AddEnvFlag(fs, ".env", "Path to the .env file")
	timeout := fs.Duration("timeout", 30*time.Second, "Command timeout")
	title := fs.String("title", "", "Updated title")
	status := fs.String("status", "", "Updated story status (story only)")
	collection := fs.String("collection", "", "Updated collection")
	urlFlag := fs.String("url", "", "Updated canonical URL")
	source := fs.String("source", "", "Updated source (article only)")
	dryRun := fs.Bool("dry-run", false, "Preview changes without applying updates")

	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "update requires exactly one UUID argument")
		printUpdateUsage()
		return 2
	}

	uuid := strings.TrimSpace(fs.Arg(0))
	if uuid == "" {
		fmt.Fprintln(os.Stderr, "UUID must not be empty")
		return 2
	}

	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})

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
		if visited["source"] {
			fmt.Fprintln(os.Stderr, "--source is only supported for `scoop update article`")
			return 2
		}
		opts := db.UpdateStoryOptions{}
		if visited["title"] {
			v := strings.TrimSpace(*title)
			opts.Title = &v
		}
		if visited["status"] {
			v := strings.TrimSpace(strings.ToLower(*status))
			opts.Status = &v
		}
		if visited["collection"] {
			v := normalizeCollectionFlag(*collection)
			opts.Collection = &v
		}
		if visited["url"] {
			v := strings.TrimSpace(*urlFlag)
			opts.URL = &v
		}
		if err := validateStoryUpdateOptions(opts); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid story update: %v\n", err)
			return 2
		}
		return runUpdateStory(ctx, pool, uuid, opts, now, *dryRun)
	default:
		if visited["status"] {
			fmt.Fprintln(os.Stderr, "--status is only supported for `scoop update story`")
			return 2
		}
		opts := db.UpdateArticleOptions{}
		if visited["title"] {
			v := strings.TrimSpace(*title)
			opts.Title = &v
		}
		if visited["source"] {
			v := strings.TrimSpace(*source)
			opts.Source = &v
		}
		if visited["collection"] {
			v := normalizeCollectionFlag(*collection)
			opts.Collection = &v
		}
		if visited["url"] {
			v := strings.TrimSpace(*urlFlag)
			opts.URL = &v
		}
		if err := validateArticleUpdateOptions(opts); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid article update: %v\n", err)
			return 2
		}
		return runUpdateArticle(ctx, pool, uuid, opts, now, *dryRun)
	}
}

func runUpdateStory(ctx context.Context, pool *db.Pool, storyUUID string, opts db.UpdateStoryOptions, now time.Time, dryRun bool) int {
	before, err := getStoryUpdateSnapshot(ctx, pool, storyUUID)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			fmt.Fprintf(os.Stderr, "Story not found: %s\n", storyUUID)
			return 1
		}
		fmt.Fprintf(os.Stderr, "Failed to load story before update: %v\n", err)
		return 1
	}

	if dryRun {
		after := *before
		if opts.Title != nil {
			after.Title = strings.TrimSpace(*opts.Title)
		}
		if opts.Status != nil {
			after.Status = strings.TrimSpace(strings.ToLower(*opts.Status))
		}
		if opts.Collection != nil {
			after.Collection = normalizeCollectionFlag(*opts.Collection)
		}
		if opts.URL != nil {
			trimmed := strings.TrimSpace(*opts.URL)
			if trimmed == "" {
				after.URL = nil
			} else {
				v := trimmed
				after.URL = &v
			}
		}
		after.UpdatedAt = now.UTC()

		fmt.Println("dry_run=true")
		if err := writeStoryUpdateDiff(*before, after); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to render diff: %v\n", err)
			return 1
		}
		return 0
	}

	if err := pool.UpdateStory(ctx, storyUUID, opts, now); err != nil {
		if errors.Is(err, db.ErrNoRows) {
			fmt.Fprintf(os.Stderr, "Story not found or already deleted: %s\n", storyUUID)
			return 1
		}
		fmt.Fprintf(os.Stderr, "Failed to update story: %v\n", err)
		return 1
	}

	after, err := getStoryUpdateSnapshot(ctx, pool, storyUUID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Story updated but failed to load post-update state: %v\n", err)
		return 1
	}

	if err := writeStoryUpdateDiff(*before, *after); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to render diff: %v\n", err)
		return 1
	}
	return 0
}

func runUpdateArticle(ctx context.Context, pool *db.Pool, articleUUID string, opts db.UpdateArticleOptions, now time.Time, dryRun bool) int {
	before, err := getArticleUpdateSnapshot(ctx, pool, articleUUID)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			fmt.Fprintf(os.Stderr, "Article not found: %s\n", articleUUID)
			return 1
		}
		fmt.Fprintf(os.Stderr, "Failed to load article before update: %v\n", err)
		return 1
	}

	if dryRun {
		after := *before
		if opts.Title != nil {
			after.Title = normalizePreviewTitle(*opts.Title)
		}
		if opts.Source != nil {
			after.Source = strings.TrimSpace(*opts.Source)
		}
		if opts.Collection != nil {
			after.Collection = normalizeCollectionFlag(*opts.Collection)
		}
		if opts.URL != nil {
			trimmed := strings.TrimSpace(*opts.URL)
			if trimmed == "" {
				after.URL = nil
				after.SourceDomain = nil
			} else {
				v := trimmed
				after.URL = &v
				after.SourceDomain = hostFromURL(trimmed)
			}
		}
		after.UpdatedAt = now.UTC()

		fmt.Println("dry_run=true")
		if err := writeArticleUpdateDiff(*before, after); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to render diff: %v\n", err)
			return 1
		}
		return 0
	}

	if err := pool.UpdateArticle(ctx, articleUUID, opts, now); err != nil {
		if errors.Is(err, db.ErrNoRows) {
			fmt.Fprintf(os.Stderr, "Article not found or already deleted: %s\n", articleUUID)
			return 1
		}
		fmt.Fprintf(os.Stderr, "Failed to update article: %v\n", err)
		return 1
	}

	after, err := getArticleUpdateSnapshot(ctx, pool, articleUUID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Article updated but failed to load post-update state: %v\n", err)
		return 1
	}

	if err := writeArticleUpdateDiff(*before, *after); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to render diff: %v\n", err)
		return 1
	}
	return 0
}

func getStoryUpdateSnapshot(ctx context.Context, pool *db.Pool, storyUUID string) (*updateStorySnapshot, error) {
	const q = `
SELECT
	s.story_uuid::text,
	s.canonical_title,
	s.status,
	s.collection,
	s.canonical_url,
	s.updated_at
FROM news.stories s
WHERE s.story_uuid = $1::uuid
  AND s.deleted_at IS NULL
LIMIT 1
`

	var snap updateStorySnapshot
	if err := pool.QueryRow(ctx, q, strings.TrimSpace(storyUUID)).Scan(
		&snap.StoryUUID,
		&snap.Title,
		&snap.Status,
		&snap.Collection,
		&snap.URL,
		&snap.UpdatedAt,
	); err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return nil, db.ErrNoRows
		}
		return nil, err
	}
	return &snap, nil
}

func getArticleUpdateSnapshot(ctx context.Context, pool *db.Pool, articleUUID string) (*updateArticleSnapshot, error) {
	const q = `
SELECT
	a.article_uuid::text,
	a.normalized_title,
	a.source,
	a.collection,
	a.canonical_url,
	a.source_domain,
	a.updated_at
FROM news.articles a
WHERE a.article_uuid = $1::uuid
  AND a.deleted_at IS NULL
LIMIT 1
`

	var snap updateArticleSnapshot
	if err := pool.QueryRow(ctx, q, strings.TrimSpace(articleUUID)).Scan(
		&snap.ArticleUUID,
		&snap.Title,
		&snap.Source,
		&snap.Collection,
		&snap.URL,
		&snap.SourceDomain,
		&snap.UpdatedAt,
	); err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return nil, db.ErrNoRows
		}
		return nil, err
	}
	return &snap, nil
}

func writeStoryUpdateDiff(before, after updateStorySnapshot) error {
	rows := [][]string{
		{"title", before.Title, after.Title},
		{"status", before.Status, after.Status},
		{"collection", before.Collection, after.Collection},
		{"url", pointerStringOrEmpty(before.URL), pointerStringOrEmpty(after.URL)},
		{"updated_at", formatUTCTimestamp(before.UpdatedAt), formatUTCTimestamp(after.UpdatedAt)},
	}
	return writeTable([]string{"field", "before", "after"}, rows)
}

func writeArticleUpdateDiff(before, after updateArticleSnapshot) error {
	rows := [][]string{
		{"title", before.Title, after.Title},
		{"source", before.Source, after.Source},
		{"collection", before.Collection, after.Collection},
		{"url", pointerStringOrEmpty(before.URL), pointerStringOrEmpty(after.URL)},
		{"source_domain", pointerStringOrEmpty(before.SourceDomain), pointerStringOrEmpty(after.SourceDomain)},
		{"updated_at", formatUTCTimestamp(before.UpdatedAt), formatUTCTimestamp(after.UpdatedAt)},
	}
	return writeTable([]string{"field", "before", "after"}, rows)
}

func validateStoryUpdateOptions(opts db.UpdateStoryOptions) error {
	if opts.Title == nil && opts.Status == nil && opts.Collection == nil && opts.URL == nil {
		return fmt.Errorf("at least one update flag is required")
	}

	if opts.Title != nil && strings.TrimSpace(*opts.Title) == "" {
		return fmt.Errorf("--title must not be empty")
	}
	if opts.Status != nil && strings.TrimSpace(*opts.Status) == "" {
		return fmt.Errorf("--status must not be empty")
	}
	if opts.Collection != nil && normalizeCollectionFlag(*opts.Collection) == "" {
		return fmt.Errorf("--collection must not be empty")
	}
	if opts.URL != nil && !isFullyQualifiedURL(*opts.URL) {
		return fmt.Errorf("--url must be a fully-qualified URL")
	}

	return nil
}

func validateArticleUpdateOptions(opts db.UpdateArticleOptions) error {
	if opts.Title == nil && opts.Source == nil && opts.Collection == nil && opts.URL == nil {
		return fmt.Errorf("at least one update flag is required")
	}

	if opts.Title != nil && strings.TrimSpace(*opts.Title) == "" {
		return fmt.Errorf("--title must not be empty")
	}
	if opts.Source != nil && strings.TrimSpace(*opts.Source) == "" {
		return fmt.Errorf("--source must not be empty")
	}
	if opts.Collection != nil && normalizeCollectionFlag(*opts.Collection) == "" {
		return fmt.Errorf("--collection must not be empty")
	}
	if opts.URL != nil && !isFullyQualifiedURL(*opts.URL) {
		return fmt.Errorf("--url must be a fully-qualified URL")
	}

	return nil
}

func isFullyQualifiedURL(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	return parsed.Scheme != "" && parsed.Host != ""
}

func normalizePreviewTitle(raw string) string {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return ""
	}
	return strings.Join(strings.Fields(trimmed), " ")
}

func hostFromURL(raw string) *string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil || strings.TrimSpace(parsed.Host) == "" {
		return nil
	}
	host := strings.TrimSpace(strings.ToLower(parsed.Hostname()))
	if host == "" {
		return nil
	}
	return &host
}

func printUpdateUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  scoop update story <story_uuid> [--title ...] [--status ...] [--collection ...] [--url ...] [--dry-run] [--env .env] [--timeout 30s]")
	fmt.Fprintln(os.Stderr, "  scoop update article <article_uuid> [--title ...] [--source ...] [--collection ...] [--url ...] [--dry-run] [--env .env] [--timeout 30s]")
}
