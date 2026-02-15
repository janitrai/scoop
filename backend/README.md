# Scoop

This project stores, normalizes, and deduplicates news items collected by OpenClaw.
OpenClaw is the collector. This CLI is the backend pipeline that turns raw items into canonical stories.

## Current State

The ingestion, normalization, embedding, and deduplication stages are implemented now.
Topic assignment and digest publishing are planned next stages.
Database access uses GORM (with PostgreSQL) and raw SQL executed through GORM sessions/transactions.
An Echo HTTP server is available for story browsing and diagnostics.
API responses use JSend envelopes.

## Story Viewer API + Web App

Local dev runs as two processes:

1. Backend API (Echo + PostgreSQL):

```bash
make dev
```

2. Frontend dev server (Vite):

```bash
pnpm --dir ../frontend dev
```

Open in browser:
- `http://127.0.0.1:5173/`

Notes:
- Backend serves API only (`/api/v1/...`).
- Vite proxies `/api/*` to `http://127.0.0.1:8090`.
- Frontend production build outputs to `../frontend/dist` (not into backend code).
- `make dev` uses `air` hot reload with config in `.air.toml`.
- One-time fallback if you prefer no watcher:

```bash
go run ./cmd/scoop serve --env .env --host 0.0.0.0 --port 8090
```

Frontend stack:
- React + Vite + strict TypeScript (source in `../frontend/`)
- Tailwind CSS (component layer via `@apply` in `../frontend/src/styles.css`)
- TanStack Router (typed route/search state)
- TanStack Query (typed API caching/fetching)

Build frontend assets:

```bash
pnpm --dir ../frontend install
pnpm --dir ../frontend build
```

Web app day browsing:
- left/right buttons to move across day buckets
- center day button opens a calendar picker
- relative date text (`today`, `yesterday`, `3 days ago`, `3 weeks ago`)
- manual `From/To` fields still available for custom ranges

Key API endpoints:
- `GET /api/v1/health`
- `GET /api/v1/stats`
- `GET /api/v1/collections`
- `GET /api/v1/story-days?collection=ai_news&limit=45`
- `GET /api/v1/stories?page=1&page_size=25&collection=ai_news&q=...&from=YYYY-MM-DD&to=YYYY-MM-DD`
- `GET /api/v1/stories/{story_uuid}`

## Embedding Service

`scoop embed` and `scoop process` call an external HTTP embedding endpoint.

Default endpoint expected by the Go pipeline:
- `http://127.0.0.1:8844/embed`

Start the service from this workspace:

```bash
cd ~/scoop/embedding-service
./.venv/bin/python main.py --server --backend transformers --host 0.0.0.0 --port 8844
```

Fast local smoke mode (no model load):

```bash
cd ~/scoop/embedding-service
./.venv/bin/python main.py --server --backend deterministic --host 0.0.0.0 --port 8844
```

Health check:

```bash
curl -s http://127.0.0.1:8844/health
```

## Collection + ID Tagging

To tag which scrape operation produced an item, use `source_metadata.collection` as a plain string.
Examples: `ai_news`, `world_news`, `china_news`.
This value is a hard dedup boundary: items only deduplicate against stories in the same collection.

Required metadata keys in payload:
- `source_metadata.collection`
- `source_metadata.job_name`
- `source_metadata.job_run_id`
- `source_metadata.scraped_at` (RFC3339)

Also pass `--triggered-by-topic` to `scoop ingest` for run-level tagging in `ingest_runs`.

UUID conventions:
- `source_metadata.scrape_run_uuid`: one UUID per OpenClaw scrape run. Every item from that run carries the same value.
- `source_metadata.item_uuid`: one stable UUID per logical source item.
  - recommended generation: deterministic UUIDv5 over `source + ":" + source_item_id`
  - keep it stable across retries/re-scrapes of the same source item
- `story_uuid`: UUID handle for canonical deduplicated stories (`news.stories.story_uuid`).

## Plain-Language Pipeline

1. **Collect + Submit**
- OpenClaw browses the web.
- OpenClaw builds canonical JSON per article.
- OpenClaw calls `scoop ingest`.

2. **Validate + Store Raw**
- Check payload schema (`v1` required fields).
- Save immutable raw row (`raw_arrivals`).
- Save run/checkpoint metadata (`ingest_runs`, `source_checkpoints`).
- Skip exact raw duplicates by payload hash key.

3. **Normalize**
- Clean URL/text/date/language.
- Remove tracking params from URL.
- Compute stable hashes (URL/content/title).
- Save canonical document row (`documents`).

4. **Enrich (Embeddings)**
- Create semantic vectors for documents.
- Save vectors to `document_embeddings`.
- Status: implemented.

5. **Deduplicate (all dedup logic in one place)**
- Scope boundary:
  - only compare against existing stories in the same `collection`
- Exact dedup:
  - same canonical URL
  - same `source + source_item_id`
  - same content hash
- Lexical auto-merge:
  - title simhash distance `<= 3` bits
  - or title trigram Jaccard `>= 0.88` with publish-date delta `<= 14d`
- Semantic dedup (embeddings):
  - search top `K=20` candidate stories in a configurable lookback window (default `365` days)
  - auto-merge if cosine `>= 0.965`, or cosine `>= 0.935` and title overlap `>= 0.30`
  - mark gray-zone if best cosine is in `[0.89, 0.935)`; create a new story but keep candidate evidence in `dedup_events`
- Decision:
  - `auto_merge`, `new_story`, or `gray_zone`
- Save outputs:
  - `stories`
  - `story_members`
  - `dedup_events` (audit trail)
- Useful flags:
  - `scoop dedup --lookback-days <N>`
  - `scoop process --dedup-lookback-days <N>`

6. **Topic + Digest + Publish**
- Assign stories to topics.
- Build daily digest candidates and rankings.
- Publish digest to channel(s).
- Status: planned, not implemented yet in Go CLI.

7. **Observe + Recover**
- Track counts/decisions in DB + logs.
- If publish path fails during migration, use fallback path.

In short: OpenClaw finds articles, this pipeline stores them safely, deduplicates them into canonical stories, and prepares the data model for digest/report publishing.

## Dedup Performance Testing

Use the built-in benchmark harness for repeatable dedup timing:

```bash
make bench-dedup-perf
```

What it does:
- Resets pipeline tables to a clean state.
- Ingests all JSON files under `testdata/scraped_news/`.
- Runs `normalize`, `embed` (via local deterministic mock embedding server), and `dedup`.
- Reports timings, dedup throughput (`docs/sec`), table counts, and decision breakdown.
- Cleans benchmark data afterward by default.

Useful options:

```bash
bash scripts/bench_dedup_performance.sh --lookback-days 90
bash scripts/bench_dedup_performance.sh --keep-data
bash scripts/bench_dedup_performance.sh --dataset-dir testdata/scraped_news
```

## Ground Truth Evaluation

Manual annotations for all scraped fixtures live in:

- `testdata/ground_truth/dedup_ground_truth_items.jsonl`
- `testdata/ground_truth/dedup_ground_truth_meta.json`

To evaluate current dedup output against that ground truth:

```bash
make eval-dedup-gt
```

Optional strict threshold:

```bash
bash scripts/eval_dedup_ground_truth.sh --min-f1 0.98
```
