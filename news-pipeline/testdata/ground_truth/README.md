# Dedup Ground Truth

This folder contains manually reviewed dedup annotations for all files under `testdata/scraped_news/`.

## Files

- `dedup_ground_truth_items.jsonl`
  - One JSON object per scraped file.
  - Key fields:
    - `file`: path to source fixture file (primary key)
    - `story_gt_id`: canonical ground-truth story ID
    - `source`, `source_item_id`, `item_uuid`, `canonical_url`, `normalized_canonical_url`, `title`, `collection`
- `dedup_ground_truth_meta.json`
  - Dataset-level metadata and review notes.
  - Includes manual merge rules and summary counts.

## Annotation Method

Base clustering uses strict identity unions:
- normalized canonical URL
- `source_metadata.item_uuid`
- `source + source_item_id`

Then manual review applies semantic merges for known same-event cases across different URLs.

## Scope

- Coverage: all scraped files in `testdata/scraped_news/` (currently 516 items).
- Output granularity: story-level dedup clusters for evaluation and regression testing.
