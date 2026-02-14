package pipeline

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type groundTruthItem struct {
	File      string `json:"file"`
	StoryGTID string `json:"story_gt_id"`
}

type groundTruthMeta struct {
	ItemsTotal   int `json:"items_total"`
	StoriesTotal int `json:"stories_total"`
	ManualMerges []struct {
		ID    string   `json:"id"`
		Files []string `json:"files"`
	} `json:"manual_merge_rules"`
}

func TestGroundTruthAnnotations_CoverageAndUniqueness(t *testing.T) {
	t.Parallel()

	scrapedFiles, err := filepath.Glob(filepath.FromSlash("../../testdata/scraped_news/*/*.json"))
	if err != nil {
		t.Fatalf("glob scraped files: %v", err)
	}
	if len(scrapedFiles) == 0 {
		t.Fatalf("expected scraped files to exist")
	}

	gtPath := filepath.FromSlash("../../testdata/ground_truth/dedup_ground_truth_items.jsonl")
	f, err := os.Open(gtPath)
	if err != nil {
		t.Fatalf("open ground truth items: %v", err)
	}
	defer f.Close()

	fileSeen := make(map[string]struct{}, len(scrapedFiles))
	storySeen := map[string]struct{}{}
	line := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line++
		var item groundTruthItem
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			t.Fatalf("decode ground truth line %d: %v", line, err)
		}
		normalizedFile := normalizeFixturePath(item.File)
		if normalizedFile == "" {
			t.Fatalf("line %d: file is empty", line)
		}
		if item.StoryGTID == "" {
			t.Fatalf("line %d: story_gt_id is empty", line)
		}
		if _, exists := fileSeen[normalizedFile]; exists {
			t.Fatalf("duplicate annotation for file %q", normalizedFile)
		}
		fileSeen[normalizedFile] = struct{}{}
		storySeen[item.StoryGTID] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan ground truth: %v", err)
	}

	for _, file := range scrapedFiles {
		normalizedFile := normalizeFixturePath(file)
		if _, ok := fileSeen[normalizedFile]; !ok {
			t.Fatalf("missing ground truth annotation for file %q", file)
		}
	}
	if len(fileSeen) != len(scrapedFiles) {
		t.Fatalf("annotation count mismatch: have=%d want=%d", len(fileSeen), len(scrapedFiles))
	}
	if len(storySeen) == 0 {
		t.Fatalf("expected non-zero story ground truth IDs")
	}
}

func normalizeFixturePath(path string) string {
	normalized := filepath.ToSlash(filepath.Clean(path))
	normalized = strings.TrimPrefix(normalized, "../../")
	normalized = strings.TrimPrefix(normalized, "./")
	return normalized
}

func TestGroundTruthMeta_ConsistentWithItems(t *testing.T) {
	t.Parallel()

	metaBytes, err := os.ReadFile(filepath.FromSlash("../../testdata/ground_truth/dedup_ground_truth_meta.json"))
	if err != nil {
		t.Fatalf("read ground truth meta: %v", err)
	}
	var meta groundTruthMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("decode ground truth meta: %v", err)
	}

	itemsFile, err := os.Open(filepath.FromSlash("../../testdata/ground_truth/dedup_ground_truth_items.jsonl"))
	if err != nil {
		t.Fatalf("open ground truth items: %v", err)
	}
	defer itemsFile.Close()

	itemCount := 0
	storyIDs := map[string]struct{}{}
	scanner := bufio.NewScanner(itemsFile)
	for scanner.Scan() {
		var item groundTruthItem
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			t.Fatalf("decode item line: %v", err)
		}
		itemCount++
		storyIDs[item.StoryGTID] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan items: %v", err)
	}

	if meta.ItemsTotal != itemCount {
		t.Fatalf("meta items_total mismatch: have=%d want=%d", meta.ItemsTotal, itemCount)
	}
	if meta.StoriesTotal != len(storyIDs) {
		t.Fatalf("meta stories_total mismatch: have=%d want=%d", meta.StoriesTotal, len(storyIDs))
	}
	if len(meta.ManualMerges) == 0 {
		t.Fatalf("expected manual_merge_rules in meta")
	}
	for _, rule := range meta.ManualMerges {
		if rule.ID == "" {
			t.Fatalf("manual merge rule has empty id")
		}
		if len(rule.Files) < 2 {
			t.Fatalf("manual merge rule %q must include at least 2 files", rule.ID)
		}
	}
}
