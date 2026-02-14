# Setup Log

## 2026-02-13T13:40:57+01:00

- Task: Apply schema from `IMPLEMENTATION_PLAN.md` section `1. Complete SQL Schema (DDL)` to `news_pipeline` as user `news`.
- Extracted SQL block to `/tmp/news_pipeline_schema.sql` (348 lines).
- Apply command run:
  - `source .env && PGPASSWORD="$PGPASSWORD" psql -v ON_ERROR_STOP=1 -X -h localhost -U news -d news_pipeline -f /tmp/news_pipeline_schema.sql`
- Apply result:
  - Aborted at `CREATE TYPE ingest_run_status` with `ERROR: type "ingest_run_status" already exists`.
  - This indicates schema objects were already present.
- Table verification in schema `news`:
  - Required tables checked: `ingest_runs`, `source_checkpoints`, `raw_arrivals`, `documents`, `document_embeddings`, `stories`, `story_members`, `dedup_events`, `topics`, `topic_source_rules`, `topic_keyword_rules`, `story_topic_state`, `digest_runs`, `digest_entries`.
  - Result: all 14 tables are present.
- Smoke test (`news.ingest_runs`):
  - Inserted dummy row: `run_id=1`, `source=smoke_test_1770986452`, `triggered_by_topic=setup_smoke`.
  - Queried row back successfully.
  - Deleted row successfully (`DELETE 1`).
  - Post-delete verification: `count(*) = 0`.

Status: Completed. Schema is present, table verification passed, and ingest smoke test passed.

## 2026-02-13T14:41:04+01:00

- Task: Bootstrap Go backend implementation for `news-pipeline` (using `~/horse/backend` conventions) and ship first working commands.
- Added Go project/tooling files:
  - `go.mod`, `.gitignore`, `.golangci.yml`, `Makefile`
  - `cmd/news-pipeline/main.go`
  - `internal/app` command handlers for `health` and `ingest`
  - `internal/cli/env.go` (`--env` loader pattern)
  - `internal/config/config.go` (envconfig-based config)
  - `internal/logging/logger.go` (zerolog setup)
  - `internal/db/pool.go` (pgxpool creation and ping)
  - `internal/globaltime/time.go` (testable time wrapper)
  - `internal/ingest/service.go` (transactional writes to `news.ingest_runs`, `news.raw_arrivals`, `news.source_checkpoints`)
- Installed local Go toolchain:
  - `/home/bob/.local/toolchains/go1.26.0/go/bin/go`
- Verification executed:
  - `go mod tidy`
  - `go fmt ./...`
  - `go test ./...`
  - `go vet ./...`
  - `go build -o /tmp/news-pipeline ./cmd/news-pipeline`
  - `/tmp/news-pipeline health --env .env` -> `ok: database ping successful`
- Ingest smoke execution:
  - `/tmp/news-pipeline ingest --env .env --source smoke_go_cli --source-item-id go-smoke-1770990053 --payload '{"title":"Go smoke ingest","id":"go-smoke-1770990053"}' --checkpoint '{"cursor":"go-smoke-1770990053"}' --triggered-by-topic setup_smoke`
  - Output included:
    - `run_id=3 status=completed inserted=true ...`
    - `raw_arrival_id=2`
  - DB verification:
    - `news.ingest_runs`: `3|completed|1`
    - `news.raw_arrivals`: `2|go-smoke-1770990053`
  - Cleanup completed:
    - deleted `raw_arrivals` row (`raw_arrival_id=2`)
    - deleted `ingest_runs` row (`run_id=3`)
    - deleted `source_checkpoints` row for source `smoke_go_cli`
    - post-cleanup counts: `0` for both checked rows

Status: Go bootstrap complete and verified with live DB read/write smoke run.

## 2026-02-13T18:12:03+01:00

- Task: Document canonical news item schema standard decision.
- Added `NEWS_ITEM_SCHEMA.md`:
  - Selected internal canonical schema with `schema.org/NewsArticle` semantics.
  - Chose JSON Schema Draft 2020-12 for ingest validation.
  - Positioned Dublin Core as optional mapping/import-export only.
  - Defined v1 required/optional fields, mapping guidance, and follow-up tasks.
- Updated `IMPLEMENTATION_PLAN.md`:
  - Added section `7. Canonical News Item Schema Standard`.
  - Linked to `NEWS_ITEM_SCHEMA.md` as source of truth.

Status: Schema standard decision documented and committed to project docs.

## 2026-02-13T18:14:59+01:00

- Task: Implement canonical news-item schema validation in Go ingest flow.
- Added schema file:
  - `schema/news_item.schema.json` (Draft 2020-12)
- Added validation package:
  - `schema/validator.go`
  - `schema/validator_test.go`
  - Uses `github.com/santhosh-tekuri/jsonschema/v5` with format assertions enabled.
- Ingest command update:
  - `internal/app/ingest.go` now validates payload against canonical schema before DB writes.
  - Enforces consistency between CLI overrides and payload identity fields:
    - `--source` must match payload `source` (if provided)
    - `--source-item-id` must match payload `source_item_id` (if provided)
    - `--source-item-url` must match payload `canonical_url` (if both provided)
    - `--published-at` must match payload `published_at` (if both provided)
  - Defaults now use schema-valid payload shape (`payload_version=v1`, required fields present).
- Tooling updates:
  - `go.mod`/`go.sum` updated for JSON schema validator dependency.
  - `Makefile` ingest smoke command updated to send schema-valid payload.
- Verification executed:
  - `go mod tidy`
  - `go fmt ./...`
  - `go test ./...`
  - `go vet ./...`
  - `go build -o /tmp/news-pipeline ./cmd/news-pipeline`
  - `/tmp/news-pipeline health --env .env` -> passed
  - Live ingest smoke:
    - inserted run `run_id=4`, `raw_arrival_id=3`, status `completed`
    - verified rows in `news.ingest_runs` and `news.raw_arrivals`
    - cleaned up inserted smoke rows and checkpoint (post-cleanup counts `0`)
  - Negative validation check:
    - payload missing `source_item_id` rejected with schema error and exit code `2`.

Status: Canonical schema validation is now enforced at ingest boundary and verified end-to-end.

## 2026-02-13T19:49:43+01:00

- Task: Implement modern normalize + dedup pipeline execution path in Go CLI.
- Added new commands in `news-pipeline` CLI:
  - `normalize`: processes pending `news.raw_arrivals` into `news.documents`
  - `dedup`: processes pending `news.documents` into `news.stories`, `news.story_members`, `news.dedup_events`
  - `process` (and alias `run-once`): runs normalize + dedup in cycles until queue drain (bounded by `--max-cycles`)
- Added command implementation:
  - `internal/app/pipeline.go`
  - wired in `internal/app/app.go`
- Added Makefile targets:
  - `run-normalize`
  - `run-dedup`
  - `run-process`
- Existing modern pipeline service code lives in:
  - `internal/pipeline/service.go`
  - Includes URL/content exact-match dedup, lexical overlap scoring, gray-zone tracking, and dedup audit events.

- Verification executed:
  - `/home/bob/.local/toolchains/go1.26.0/go/bin/go fmt ./...`
  - `/home/bob/.local/toolchains/go1.26.0/go/bin/go test ./...`
  - `/home/bob/.local/toolchains/go1.26.0/go/bin/go vet ./...`

- Live DB modern dedup smoke test:
  - Inserted 2 schema-valid ingest items with different sources/item IDs but equivalent canonical URLs (tracking params differed).
  - Ran:
    - `go run ./cmd/news-pipeline process --env .env --normalize-limit 100 --dedup-limit 100 --until-empty=true --max-cycles 5`
  - Process output:
    - cycle 1: `normalize_processed=2`, `dedup_processed=2`, `new_stories=1`, `auto_merges=1`
    - cycle 2: no pending work, drained.
  - DB verification:
    - `documents=2`
    - `stories=1`
    - `story_members=2`
    - `dedup_events=2`
    - decisions: `auto_merge,new_story`
  - Cleanup completed:
    - removed smoke rows from `dedup_events`, `story_members`, `stories`, `documents`, `raw_arrivals`, `source_checkpoints`, and `ingest_runs`.

Status: Modern normalize/dedup pipeline is implemented, reachable via CLI, and verified against live Postgres with dedup behavior and cleanup.

## 2026-02-14T12:05:17+01:00

- Task: Implement embedding worker and semantic dedup integration in Go pipeline.
- Added embedding stage in pipeline service:
  - `internal/pipeline/embed.go`
  - New `EmbedOptions` / `EmbedResult`
  - New command defaults:
    - endpoint: `http://127.0.0.1:8844/embed`
    - model: `Qwen3-Embedding-8B`
    - model version: `v1`
    - batch size: `32`
  - Writes vectors to `news.document_embeddings` using pgvector (`vector(4096)`).
- Updated dedup stage to use embeddings:
  - `DedupPending` now operates by model name/version and claims only documents with embeddings for that model.
  - Added semantic candidate lookup via cosine distance against representative story embeddings.
  - Added semantic auto-merge rules:
    - cosine `>= 0.965` OR
    - cosine `>= 0.935` and title overlap `>= 0.30`
  - Added semantic composite score for gray-zone evaluation:
    - `0.75*cosine + 0.15*title_overlap + 0.10*entity_date_consistency`
  - Dedup events now persist semantic metrics (`best_cosine`, overlap/date/composite fields, `exact_signal=semantic` for semantic auto-merges).
- Added CLI wiring:
  - `internal/app/app.go`: new `embed` command and usage updates.
  - `internal/app/pipeline.go`:
    - new `runEmbed`
    - `runDedup` takes `--model-name` and `--model-version`
    - `runProcess` now runs normalize -> embed -> dedup each cycle
      with embed flags (`--embed-limit`, `--embed-batch-size`, `--embed-endpoint`, `--embed-max-length`, `--embed-request-timeout`).
- Added tests:
  - `internal/pipeline/embed_test.go` (endpoint normalization, vector dimension validation, semantic threshold helpers).
- Updated Makefile:
  - added `run-embed`
  - updated `run-process` to include embed stage flags and naming.

- Verification executed:
  - `go fmt ./...`
  - `go test ./...`
  - `go vet ./...`
  - `go run ./cmd/news-pipeline --help` (includes `embed`)
  - `go run ./cmd/news-pipeline embed -h`
  - `go run ./cmd/news-pipeline process -h`

- Semantic end-to-end smoke test (with temporary local mock embedding server on `127.0.0.1:18844`):
  - Ingested 2 distinct items (no exact URL/source/content match).
  - Ran `normalize`, `embed` (`model=smoke-embed`), then `dedup`.
  - Observed:
    - `documents=2`
    - `embeddings=2`
    - `stories=1`
    - `dedup_events=2`
    - decisions: `new_story,auto_merge`
    - signals included `semantic` for auto-merge event.
  - Cleaned all inserted smoke rows (`dedup_events`, `story_members`, `stories`, `document_embeddings`, `documents`, `raw_arrivals`, `source_checkpoints`, `ingest_runs`).

Status: Full normalize + embed + semantic dedup path is implemented and verified end-to-end.

## 2026-02-14T12:11:53+01:00

- Task: Complete full embedding-based dedup behavior (top-K semantic retrieval + lookback window + production lexical gates).
- Dedup algorithm updates (`internal/pipeline/service.go`):
  - Added dedup lookback option (`DefaultDedupLookbackDays=365`) and passed it through `DedupOptions`.
  - Dedup candidate set now constrained by lookback window for lexical and semantic retrieval.
  - Semantic dedup now retrieves top-K candidates (`K=20`) from pgvector instead of a single nearest row.
  - Added `SET LOCAL hnsw.ef_search = 64` for semantic candidate search quality.
  - Auto-merge semantic rules:
    - cosine `>= 0.965`, or
    - cosine `>= 0.935` with title overlap `>= 0.30`.
  - Gray-zone semantic rule:
    - cosine in `[0.89, 0.935)` -> create new story + record best candidate evidence in `dedup_events`.
  - Lexical auto-merge now supports:
    - title simhash Hamming distance `<= 3` (`lexical_simhash`),
    - title trigram Jaccard `>= 0.88` with publish-date delta `<= 14 days` (`lexical_overlap`).
- CLI updates (`internal/app/pipeline.go`):
  - `dedup` command now has `--lookback-days`.
  - `process` command now has `--dedup-lookback-days`.
- Test coverage updates (`internal/pipeline/service_test.go`):
  - Added tests for trigram overlap, simhash distance, date-window gate, and semantic gray-zone helper.
- Docs update (`README.md`):
  - Dedup section now documents exact lexical + semantic thresholds and new lookback flags.

- Verification executed:
  - `gofmt -w internal/pipeline/service.go internal/app/pipeline.go internal/pipeline/service_test.go`
  - `go test ./...`
  - `go vet ./...`
  - `go run ./cmd/news-pipeline dedup -h`
  - `go run ./cmd/news-pipeline process -h`

- End-to-end semantic smoke test (live Postgres + temporary mock embedding service):
  - Ingested 2 schema-valid items with different source IDs/URLs/content (`source=smoke_semantic`).
  - Ran `normalize`, `embed` (`model=smoke-embed-v2`), and `dedup --lookback-days 365`.
  - Observed decisions:
    - `new_story=1`
    - `auto_merge=1`
    - `gray_zone=0`
  - Cleaned all smoke rows from `dedup_events`, `story_members`, `stories`, `document_embeddings`, `documents`, `raw_arrivals`, `source_checkpoints`, and `ingest_runs`.

Status: Dedup now runs full embedding-based candidate retrieval with production thresholds, lookback scoping, and verified end-to-end behavior.

## 2026-02-14T12:31:31+01:00

- Task: Add repeatable dedup performance testing harness and run a baseline benchmark.
- Added benchmark script:
  - `scripts/bench_dedup_performance.sh`
  - Behavior:
    - resets pipeline tables,
    - ingests `testdata/scraped_news/*.json`,
    - runs normalize + embed + dedup,
    - uses a deterministic local mock embedding endpoint,
    - reports stage timings + dedup throughput + decision counts,
    - cleans benchmark data by default (override with `--keep-data`).
- Added Make target:
  - `make bench-dedup-perf`
- Updated docs:
  - `README.md` now includes a `Dedup Performance Testing` section and usage examples.

- Baseline benchmark run executed (`lookback_days=365`, dataset size `516`):
  - `timing_ingest_ms=8050`
  - `timing_normalize_ms=489`
  - `timing_embed_ms=1586`
  - `timing_dedup_ms=15575`
  - `timing_dedup_empty_ms=13`
  - `dedup_processed_docs=516`
  - `dedup_throughput_docs_per_sec=33.13`
  - decisions:
    - `new_story=358`
    - `auto_merge=158`
    - `gray_zone=0`
- Post-run verification:
  - benchmark cleanup ran successfully; pipeline tables returned to zero rows.

Status: Dedup performance can now be tested repeatably with one command and compared across code changes.

## 2026-02-14T12:44:44+01:00

- Task: Create manual ground-truth annotations over scraped corpus and add dedup evaluation tests/harness.

- Ground-truth annotation artifacts added:
  - `testdata/ground_truth/dedup_ground_truth_items.jsonl`
    - one row per scraped file (`file`, `story_gt_id`, source metadata, canonical URL, title).
  - `testdata/ground_truth/dedup_ground_truth_meta.json`
    - corpus summary + annotation method + explicit manual merge rules.
  - `testdata/ground_truth/README.md`
    - format and method documentation.

- Annotation coverage/results:
  - reviewed corpus: `516` items (`testdata/scraped_news/`).
  - ground-truth stories: `356`.
  - duplicate items beyond first occurrence: `160`.
  - manual semantic merges applied after review of duplicate clusters and high-title-similarity candidates: `4` rules.

- Automated tests added:
  - `internal/pipeline/ground_truth_test.go`
    - validates full coverage (all scraped files annotated exactly once), uniqueness, and metadata consistency.

- Evaluation/benchmark scripts added:
  - `scripts/eval_dedup_ground_truth.sh`
    - ingests dataset, runs normalize+embed+dedup, maps `raw_arrival_id -> file`, compares predicted stories to ground truth.
    - computes pairwise precision/recall/F1 and supports `--min-f1` gate (default `0.99`).
  - Existing perf harness kept:
    - `scripts/bench_dedup_performance.sh`

- Makefile targets added:
  - `make bench-dedup-perf`
  - `make eval-dedup-gt`

- README updates:
  - Added `Ground Truth Evaluation` section and command usage.

- Verification executed:
  - `go test ./...`
  - `bash scripts/eval_dedup_ground_truth.sh`

- Ground-truth evaluation result (full 516-item corpus):
  - `eval_pair_precision=1.000000`
  - `eval_pair_recall=0.984791`
  - `eval_pair_f1=0.992337` (PASS with default `--min-f1 0.99`)
  - dedup stage timing in eval run: `timing_dedup_ms=16405`

Status: Ground truth is now materialized and testable; dedup quality and performance can be regression-tested with one command.

## 2026-02-14T13:12:20+01:00

- Task: Align DB layer with `~/horse` conventions and switch `news-pipeline` from `pgx` to GORM.

- DB infrastructure changes:
  - Replaced `internal/db/pool.go` (`pgxpool`) with a GORM-backed adapter:
    - uses `gorm.io/driver/postgres` + `gorm.io/gorm`
    - keeps existing SQL execution style (`QueryRow`, `Query`, `Exec`, transactions) via a thin compatibility wrapper
    - preserves command-tag row-count behavior used by ingest/dedup logic
  - Connection pooling now configured through underlying `sql.DB`.
  - Added GORM log-level mapping from app config (`LOG_LEVEL`).

- Service migrations:
  - `internal/ingest/service.go` now uses `*db.Pool` + GORM-backed transactions.
  - `internal/pipeline/service.go` now uses `*db.Pool` + `db.Tx`.
  - `internal/pipeline/embed.go` now uses `*db.Pool`.

- Dependency updates:
  - `go.mod` / `go.sum` updated for:
    - `gorm.io/gorm`
    - `gorm.io/driver/postgres`
  - Removed direct `pgx` usage from application code.

- Runtime UUID prerequisite:
  - Applied migration:
    - `migrations/002_add_uuid_columns.sql`
  - Command:
    - `source .env && PGPASSWORD="$PGPASSWORD" psql -v ON_ERROR_STOP=1 -X -h localhost -U news -d news_pipeline -f migrations/002_add_uuid_columns.sql`

- Verification:
  - `go test ./...` passed.
  - `go run ./cmd/news-pipeline health --env .env` passed.
  - `go run ./cmd/news-pipeline ingest ...` passed with UUID outputs (`run_uuid`, `raw_arrival_uuid`).
  - Temporary smoke rows were deleted after verification.

- API stack note:
  - `news-pipeline` currently has no HTTP API surface (CLI-only), so no Gin-to-Echo migration was required in this repo.
  - README updated to document future API convention: Echo + JSend.

## 2026-02-14T13:14:40+01:00

- Task: Cleanup old/irrelevant ingest path code.

- Removed obsolete CLI override path in `internal/app/ingest.go`:
  - deleted `--source`, `--source-item-id`, `--source-item-url`, `--published-at`.
  - deleted helper functions used only for override reconciliation:
    - `resolveRequiredIdentityField`
    - `resolveOptionalField`
    - `resolveOptionalTimeField`
  - ingest command now always takes identity/timestamps directly from canonical payload.

- Updated related docs/examples:
  - `Makefile` ingest smoke target no longer passes `--source`.
  - `README.md` and `NEWS_ITEM_SCHEMA.md` updated: `story_uuid` is persisted (no longer "planned").

- Verification:
  - `go test ./...` passed.
  - Payload-only ingest smoke command passed and returned UUIDs.
  - temporary smoke rows were deleted after verification.

## 2026-02-14T14:06:53+01:00

- Task: Enforce collection-scoped deduplication boundaries (`source_metadata.collection`) from ingest through dedup.

- Schema migration added/applied:
  - Added `migrations/003_collection_scoped_dedup.sql`.
  - Applied via:
    - `source .env && PGPASSWORD="$PGPASSWORD" psql -X -v ON_ERROR_STOP=1 -h localhost -U news -d news_pipeline -f migrations/003_collection_scoped_dedup.sql`
  - Migration changes:
    - Added non-null `collection` column to:
      - `news.raw_arrivals`
      - `news.documents`
      - `news.stories`
    - Backfilled existing rows from raw payload metadata / existing links.
    - Added non-empty check constraints + collection-aware indexes.

- Pipeline behavior changes:
  - `ingest` now requires and stores `collection` as a first-class column in `raw_arrivals`.
  - `normalize` propagates `collection` into `documents`.
  - `dedup` match/search is now constrained to same collection for:
    - exact URL
    - exact source+source_item_id
    - exact content hash
    - lexical candidate search
    - semantic candidate search
  - New stories are created with explicit `stories.collection`.

- Tests/docs updates:
  - Added normalize collection propagation tests in `internal/pipeline/service_test.go`.
  - Updated docs:
    - `README.md`
    - `NEWS_ITEM_SCHEMA.md`
    - `IMPLEMENTATION_PLAN.md`

- Verification executed:
  - `go test ./...` passed.
  - Targeted smoke test:
    - inserted 2 near-identical items with different collections (`ai_news` vs `world_news`)
    - ran `normalize`, inserted embeddings, ran `dedup`
    - verified result: `2` distinct stories (no cross-collection merge)
    - cleaned temporary smoke rows.
- Ground-truth evaluation:
  - `bash scripts/eval_dedup_ground_truth.sh`
  - result: `eval_pair_f1=0.992337` (PASS, unchanged), `eval_pred_story_count=358`.

## 2026-02-14T15:22:45+01:00

- Task: Build a production-style story browsing surface with Echo API + modern web UI.

- Server implementation:
  - Added new CLI command:
    - `news-pipeline serve`
  - Added Echo-based HTTP server package:
    - `internal/httpapi/server.go`
    - `internal/httpapi/jsend.go`
    - `internal/httpapi/assets.go`
  - Embedded frontend assets:
    - `internal/httpapi/assets/index.html`
    - `internal/httpapi/assets/app.css`
    - `internal/httpapi/assets/app.js`

- API endpoints implemented (JSend responses):
  - `GET /api/v1/health`
  - `GET /api/v1/stats`
  - `GET /api/v1/collections`
  - `GET /api/v1/stories` (pagination + filtering by collection/search/time range)
  - `GET /api/v1/stories/:story_uuid` (story + member-level dedup evidence)

- Web app behavior:
  - Responsive Story Viewer with:
    - stats cards
    - collection chips
    - search + date filtering
    - paginated story list
    - story detail pane (merged member docs + dedup signals)
  - UI served from same process at `/` with API under `/api/v1/*`.

- Dependencies:
  - Added Echo stack in `go.mod`:
    - `github.com/labstack/echo/v4`

- Tooling/docs updates:
  - `Makefile`:
    - added `run-serve` target
  - `README.md`:
    - documented serve command and API endpoints

- Verification executed:
  - `go test ./...` passed.
  - Live server run check on `0.0.0.0:8090`.
  - Fetched live responses:
    - `/api/v1/health` -> success
    - `/api/v1/stats` -> success (`stories=358`, `documents=516`)
    - `/api/v1/stories?page_size=2` -> success with real story rows
    - `/api/v1/stories/{story_uuid}` -> success with member details and dedup decision
    - `/` and `/assets/*` returned expected HTML/CSS/JS.

## 2026-02-14T16:29:56+01:00

- Task: Improve UX for easy day-by-day browsing in the story viewer.

- API changes:
  - Added endpoint:
    - `GET /api/v1/story-days?collection=<slug>&limit=<N>`
  - Returns day buckets with story counts using story `last_seen_at` (Berlin day boundaries).

- Web app changes:
  - Added "Browse By Day" chip rail under filters.
  - One-click day selection now sets a single-day range (`from=day`, `to=day`).
  - Added "all days" chip to clear day filtering quickly.
  - Day chips refresh when collection changes.

- Verification:
  - `go test ./...` passed.
  - Live API checks:
    - `/api/v1/story-days?limit=5` returned expected day buckets.
    - `/api/v1/story-days?collection=ai_news&limit=5` returned collection-scoped day buckets.
  - UI checks:
    - `/` contains `Browse By Day` section and `day-chips` container.
    - `/assets/app.js` includes day-bucket logic wired to `/api/v1/story-days`.
