# Test Data Requirements

This document defines what test data is needed for reliable ingest/dedup testing, and exactly where to store it.

## Storage Location

Save all test news items under:

`news-pipeline/testdata/news_items/`

Recommended layout:

```text
testdata/
  news_items/
    exact_url/
    exact_source_id/
    exact_content/
    near_duplicate/
    unique/
    invalid/
  manifest.csv
```

## File Format

Use separate files, not JSONL.

Rules:
- One article per `.json` file.
- UTF-8 text JSON.
- Valid canonical `v1` schema unless intentionally placed in `invalid/`.

Required fields in each valid file:
- `payload_version` (must be `"v1"`)
- `source`
- `source_item_id`
- `title`
- `source_metadata` (object)
- `source_metadata.collection` (string): scrape operation/topic label, for example `ai_news`, `world_news`, `china_news`
- `source_metadata.job_name` (string): OpenClaw job name
- `source_metadata.job_run_id` (string): OpenClaw run id
- `source_metadata.scraped_at` (string, RFC3339): scrape timestamp

Required ID handles for test datasets:
- `source_metadata.scrape_run_uuid` (string, UUID): one scrape UUID shared by all items from the same scrape execution
- `source_metadata.item_uuid` (string, UUID): stable UUID per source item
  - recommended generation rule: UUIDv5 over `source + ":" + source_item_id`

Useful optional fields:
- `canonical_url`
- `published_at` (RFC3339, UTC preferred)
- `body_text`
- `language`
- `source_domain`
- `authors`
- `tags`
- `image_url`

Additional `source_metadata` keys are allowed for source-specific details.

## Naming Convention

Use stable, readable filenames:

`<group>-<source>-<sequence>.json`

Examples:
- `g001-reuters-001.json`
- `g001-ap-002.json`
- `g014-bbc-001.json`

`group` identifies items expected to belong to the same canonical story.

## Required Data Mix

Build at least one medium dataset with 120 to 200 valid items.

Target mix:
- `exact_url/`: 20 to 40 items (same canonical URL across sources/variants)
- `exact_source_id/`: 20 to 30 items (same `source + source_item_id` retry/update cases)
- `exact_content/`: 20 to 30 items (same normalized text, different URLs)
- `near_duplicate/`: 30 to 50 items (same story rewritten, overlap but not exact)
- `unique/`: 30 to 50 items (clearly unrelated stories)
- `invalid/`: 10 to 20 items (schema-invalid payloads for rejection tests)

## Manifest File

Create `testdata/manifest.csv` to define expected behavior.

Required columns:
- `file_path`
- `case_type` (`exact_url`, `exact_source_id`, `exact_content`, `near_duplicate`, `unique`, `invalid`)
- `expected_group` (story group label, for example `g001`; empty for invalid)
- `notes`

Example rows:

```csv
file_path,case_type,expected_group,notes
news_items/exact_url/g001-reuters-001.json,exact_url,g001,canonical_url matches AP variant
news_items/exact_url/g001-ap-002.json,exact_url,g001,same story as Reuters item
news_items/near_duplicate/g014-bbc-001.json,near_duplicate,g014,rewritten summary
news_items/unique/g090-local-001.json,unique,g090,unrelated local story
news_items/invalid/bad-missing-title-001.json,invalid,,missing required title
```

## Quality Rules

For dedup quality checks:
- Keep `published_at` within 0 to 48 hours for most duplicate groups.
- Include some borderline near-duplicates with weak title overlap.
- Include some same-topic but different-story items to catch false merges.
- Include some cross-collection duplicates (same story text/url but different `source_metadata.collection`) to verify they do **not** merge.
- Keep `source_item_id` unique except in `exact_source_id/` scenarios.

## Minimum Starter Pack

If you want a quick smoke dataset first, create 30 files:
- 8 exact URL duplicates
- 6 exact source-ID duplicates
- 6 exact content duplicates
- 6 near-duplicates
- 4 unique

Then expand to the medium dataset above before tuning thresholds.

## Validator Command

OpenClaw (or any caller) can validate a dataset directory with:

```bash
news-pipeline validate --dir testdata/news_items --recursive=true
```

Behavior:
- Exit code `0`: all scanned files are valid.
- Exit code `1`: at least one invalid file, no files found, or runtime validation error.
- Prints a summary line:
  - `validate scanned=<n> valid=<n> invalid=<n> ...`
- Prints one `INVALID <path>: <error>` line per failing file.
