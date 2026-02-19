package app

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"horse.fit/scoop/internal/cli"
	"horse.fit/scoop/internal/translation"
)

func runTranslate(args []string) int {
	if len(args) == 0 {
		printTranslateUsage()
		return 2
	}

	target := strings.ToLower(strings.TrimSpace(args[0]))
	switch target {
	case "story", "article", "collection":
	default:
		fmt.Fprintf(os.Stderr, "Unknown translate target: %s\n\n", args[0])
		printTranslateUsage()
		return 2
	}

	fs := flag.NewFlagSet("translate "+target, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	envLoader := cli.AddEnvFlag(fs, ".env", "Path to the .env file")
	timeout := fs.Duration("timeout", 2*time.Minute, "Command timeout")
	lang := fs.String("lang", "", "Target language (ISO 639-1, for example: en, zh)")
	provider := fs.String("provider", "", "Translation provider name (for example: local, google)")
	dryRun := fs.Bool("dry-run", false, "Preview work without calling the translation provider")
	force := fs.Bool("force", false, "Retranslate even when cached translation exists")

	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "translate requires one argument")
		printTranslateUsage()
		return 2
	}

	targetLang := normalizeLanguageFlag(*lang)
	if targetLang == "" {
		fmt.Fprintln(os.Stderr, "--lang is required and must be a valid language code")
		return 2
	}

	identifier := strings.TrimSpace(fs.Arg(0))
	if identifier == "" {
		fmt.Fprintln(os.Stderr, "translate argument must not be empty")
		return 2
	}

	ctx, cancel, pool, err := connectReadPool(*timeout, envLoader)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer cancel()
	defer pool.Close()

	registry := translation.NewRegistryFromEnv()
	manager := translation.NewManager(pool, registry)

	runOpts := translation.RunOptions{
		TargetLang: targetLang,
		Provider:   strings.TrimSpace(*provider),
		Force:      *force,
		DryRun:     *dryRun,
	}

	resolvedProvider := strings.TrimSpace(runOpts.Provider)
	if resolvedProvider == "" {
		resolvedProvider = registry.DefaultProvider()
	}

	var stats translation.RunStats
	switch target {
	case "story":
		stats, err = manager.TranslateStoryByUUID(ctx, identifier, runOpts)
		if err != nil {
			if errors.Is(err, translation.ErrStoryNotFound) {
				fmt.Fprintf(os.Stderr, "Story not found: %s\n", identifier)
				return 1
			}
			fmt.Fprintf(os.Stderr, "Translate story failed: %v\n", err)
			return 1
		}
	case "article":
		stats, err = manager.TranslateArticleByUUID(ctx, identifier, runOpts)
		if err != nil {
			if errors.Is(err, translation.ErrArticleNotFound) {
				fmt.Fprintf(os.Stderr, "Article not found: %s\n", identifier)
				return 1
			}
			fmt.Fprintf(os.Stderr, "Translate article failed: %v\n", err)
			return 1
		}
	default:
		stats, err = manager.TranslateCollection(ctx, identifier, translation.CollectionRunOptions{
			RunOptions: runOpts,
			Progress: func(p translation.CollectionProgress) {
				fmt.Printf("Translating %d/%d stories...\n", p.Current, p.Total)
			},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Translate collection failed: %v\n", err)
			return 1
		}
	}

	fmt.Printf(
		"translate target=%s id=%s lang=%s provider=%s total=%d translated=%d cached=%d skipped=%d dry_run=%t force=%t\n",
		target,
		identifier,
		targetLang,
		resolvedProvider,
		stats.Total,
		stats.Translated,
		stats.Cached,
		stats.Skipped,
		*dryRun,
		*force,
	)
	return 0
}

func normalizeLanguageFlag(raw string) string {
	lang := strings.ToLower(strings.TrimSpace(raw))
	if lang == "" {
		return ""
	}
	lang = strings.ReplaceAll(lang, "_", "-")
	for _, r := range lang {
		if unicode.IsLetter(r) || r == '-' {
			continue
		}
		return ""
	}
	return lang
}

func printTranslateUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  scoop translate story <story_uuid> --lang <lang> [--provider local] [--dry-run] [--force] [--env .env] [--timeout 2m]")
	fmt.Fprintln(os.Stderr, "  scoop translate article <article_uuid> --lang <lang> [--provider local] [--dry-run] [--force] [--env .env] [--timeout 2m]")
	fmt.Fprintln(os.Stderr, "  scoop translate collection <name> --lang <lang> [--provider local] [--dry-run] [--force] [--env .env] [--timeout 2m]")
}
