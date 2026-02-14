package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	payloadschema "horse.fit/news-pipeline/schema"
)

type validateResult struct {
	Scanned int
	Valid   int
	Invalid int
}

func runValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	dir := fs.String("dir", "testdata/news_items", "Directory containing .json news item files")
	recursive := fs.Bool("recursive", true, "Recursively scan subdirectories")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	files, err := collectJSONFiles(strings.TrimSpace(*dir), *recursive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Validation setup failed: %v\n", err)
		return 1
	}

	result := validateResult{}
	for _, path := range files {
		result.Scanned++

		raw, err := os.ReadFile(path)
		if err != nil {
			result.Invalid++
			fmt.Fprintf(os.Stderr, "INVALID %s: read failed: %v\n", path, err)
			continue
		}

		if !json.Valid(raw) {
			result.Invalid++
			fmt.Fprintf(os.Stderr, "INVALID %s: malformed JSON\n", path)
			continue
		}

		if _, err := payloadschema.ValidateNewsItemPayload(json.RawMessage(raw)); err != nil {
			result.Invalid++
			fmt.Fprintf(os.Stderr, "INVALID %s: %v\n", path, err)
			continue
		}

		result.Valid++
	}

	fmt.Printf(
		"validate scanned=%d valid=%d invalid=%d dir=%s recursive=%t\n",
		result.Scanned,
		result.Valid,
		result.Invalid,
		strings.TrimSpace(*dir),
		*recursive,
	)

	if result.Scanned == 0 {
		fmt.Fprintf(os.Stderr, "Validation failed: no .json files found under %s\n", strings.TrimSpace(*dir))
		return 1
	}
	if result.Invalid > 0 {
		return 1
	}
	return 0
}

func collectJSONFiles(root string, recursive bool) ([]string, error) {
	cleanRoot := strings.TrimSpace(root)
	if cleanRoot == "" {
		return nil, fmt.Errorf("directory path is empty")
	}

	info, err := os.Stat(cleanRoot)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", cleanRoot, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", cleanRoot)
	}

	var files []string
	if !recursive {
		entries, err := os.ReadDir(cleanRoot)
		if err != nil {
			return nil, fmt.Errorf("read directory %s: %w", cleanRoot, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			if strings.EqualFold(filepath.Ext(name), ".json") {
				files = append(files, filepath.Join(cleanRoot, name))
			}
		}
		sort.Strings(files)
		return files, nil
	}

	err = filepath.WalkDir(cleanRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != cleanRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".json") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk directory %s: %w", cleanRoot, err)
	}

	sort.Strings(files)
	return files, nil
}
