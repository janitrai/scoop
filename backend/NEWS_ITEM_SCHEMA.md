# News Item Schema Decision

Date: 2026-02-13
Status: Accepted for v1

## Decision

For the ingestion pipeline, we will use:

1. An internal canonical news item schema owned by this repo.
2. Field semantics based on `schema.org/NewsArticle`.
3. JSON Schema Draft 2020-12 validation at the ingest boundary.
4. Raw source payload preserved unchanged in `news.raw_arrivals.raw_payload`.
5. Dublin Core as optional import/export mapping only (not the core internal schema).

Rationale:
- Dublin Core is useful but too minimal for modern dedup + ranking workflows.
- `NewsArticle` gives stronger, widely-understood field semantics for web/news data.
- A strict internal schema avoids source-specific drift while still allowing extensions.

## Canonical v1 Shape

Required fields:
- `source` (string)
- `source_item_id` (string)
- `title` (string)
- `payload_version` (string, set to `v1`)
- `source_metadata` (object; must include `collection`, `job_name`, `job_run_id`, `scraped_at`)

Optional fields:
- `canonical_url` (string, URI)
- `published_at` (string, RFC3339 timestamp)
- `body_text` (string)
- `language` (string, BCP-47 style, default `und`)
- `source_domain` (string)
- `authors` (array of strings)
- `tags` (array of strings)
- `image_url` (string, URI)

Notes:
- `source_metadata` is the extension zone for source-specific fields that do not fit the core model.
- `canonical_url` should be normalized before hashing and dedup checks.
- `published_at` should be converted to UTC before writing normalized records.
- For v1 compatibility, extra ID handles can be stored in `source_metadata`.

## Collection/Job Tagging Convention

To trace where an item came from (for example `ai_news`, `world_news`, `china_news`), store tags in `source_metadata`.

Required keys:
- `collection` (string): logical collection/topic label such as `ai_news`.
  - This is a hard dedup boundary in v1; stories only merge within the same collection.
- `job_name` (string): OpenClaw job name.
- `job_run_id` (string): unique OpenClaw run identifier.
- `scraped_at` (string, RFC3339): scrape timestamp.

ID keys (recommended now):
- `scrape_run_uuid` (string, UUID): one UUID per scrape run/job execution.
- `item_uuid` (string, UUID): stable UUID per logical source item.
  - Recommended derivation: deterministic UUIDv5 over `source + ":" + source_item_id`.

Dedup output ID:
- `story_uuid` (string, UUID): canonical story handle persisted in `news.stories.story_uuid`.

Example:

```json
{
  "payload_version": "v1",
  "source": "hacker_news",
  "source_item_id": "abc123",
  "title": "Example headline",
  "source_metadata": {
    "collection": "ai_news",
    "job_name": "openclaw-ai-daily",
    "job_run_id": "run_2026_02_14_001",
    "scraped_at": "2026-02-14T10:00:00Z",
    "scrape_run_uuid": "7f2bf2bc-3f5e-4f1f-b8cc-8f8df46ec471",
    "item_uuid": "61ab34f4-d129-5f5f-9b00-4872d06f3e95"
  }
}
```

Additionally, pass `--triggered-by-topic` at ingest time for run-level tracing in `news.ingest_runs.triggered_by_topic`.

## Source Mapping Guidance

Preferred mapping order:

1. If source provides `schema.org/NewsArticle` JSON-LD:
- `headline` -> `title`
- `datePublished` -> `published_at`
- `url` / `mainEntityOfPage` -> `canonical_url`
- `articleBody` -> `body_text`
- `author` -> `authors`
- `keywords` -> `tags`

2. If source provides Dublin Core metadata:
- `dc:title` -> `title`
- `dc:creator` -> `authors`
- `dc:date` -> `published_at`
- `dc:identifier` -> `canonical_url` (when URI)
- `dc:language` -> `language`
- `dc:subject` -> `tags`

3. Keep unmapped source fields in `source_metadata`.

## Validation Rules (v1)

At ingest command boundary:
- Reject if required fields are missing or empty.
- Reject invalid JSON.
- Reject invalid `published_at` format.
- Reject malformed URLs where URL fields are provided.
- Enforce max lengths for text fields to avoid pathological payloads.

At normalize stage:
- Normalize URL and language.
- Compute deterministic content hashes from normalized text/title.
- Preserve original raw payload separately for audit/debug.

## Versioning

- Each normalized item includes `payload_version`.
- Initial version is `v1`.
- Breaking schema changes require a new version (`v2`, etc.) plus migration logic.

## Implementation Follow-Ups

1. Add `schema/news_item.schema.json` (Draft 2020-12).
2. Validate ingest payload against this schema in `internal/app/ingest.go` before DB writes.
3. Add per-source mapper functions that output canonical v1 objects.
4. Add tests for valid/invalid payloads and mapper edge cases.
