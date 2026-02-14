# News Ingestion and Deduplication Pipeline - Implementation Plan

Updated: 2026-02-13
Target host: NVIDIA Jetson AGX Thor (aarch64, Ubuntu 24.04, 119 GB unified RAM)
Embedding service: local Qwen3-Embedding-8B HTTP endpoint on `http://127.0.0.1:8844` (4096-dim)
Timezone for digest boundaries: `Europe/Berlin`

## 0. Scope and Success Criteria

This plan migrates from the current per-topic fetch + JSONL dedup model to a single shared ingestion and canonical story pipeline, while keeping existing cron digests operational during transition.

Success criteria for v1:
- Existing topic digests still post on time during migration (`AI 09:00`, `World 09:00`, `China 10:00` Berlin).
- Every fetched item is written to an append-only ledger.
- Dedup runs within `source_metadata.collection` boundaries (for example `ai_news`, `world_news`, `china_news`).
- Digest query returns one row per canonical `story_id` (no duplicates in same digest).
- Rollback path exists at every phase.

## 1. Complete SQL Schema (DDL)

The following is a complete initial schema (single migration file, e.g. `migrations/001_initial_schema.sql`).

Note: current implementation also includes a follow-up migration to enforce collection-scoped dedup (`migrations/003_collection_scoped_dedup.sql`), adding `collection` columns on `raw_arrivals`, `documents`, and `stories`.

```sql
BEGIN;

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE SCHEMA IF NOT EXISTS news;
SET search_path = news, public;

CREATE TYPE ingest_run_status AS ENUM ('running', 'completed', 'failed');
CREATE TYPE story_match_type AS ENUM (
    'seed',
    'exact_url',
    'exact_source_id',
    'exact_content_hash',
    'lexical_simhash',
    'lexical_overlap',
    'semantic',
    'manual'
);
CREATE TYPE dedup_decision AS ENUM ('new_story', 'auto_merge', 'gray_zone', 'manual_merge', 'manual_split');
CREATE TYPE topic_rule_type AS ENUM ('include', 'exclude');
CREATE TYPE digest_run_status AS ENUM ('running', 'completed', 'failed');
CREATE TYPE digest_entry_status AS ENUM (
    'new',
    'seen',
    'suppressed_duplicate',
    'suppressed_manual',
    'possible_duplicate'
);

CREATE OR REPLACE FUNCTION touch_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE TABLE ingest_runs (
    run_id BIGSERIAL PRIMARY KEY,
    source TEXT NOT NULL CHECK (length(trim(source)) > 0),
    triggered_by_topic TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    status ingest_run_status NOT NULL DEFAULT 'running',
    items_fetched INTEGER NOT NULL DEFAULT 0 CHECK (items_fetched >= 0),
    items_inserted INTEGER NOT NULL DEFAULT 0 CHECK (items_inserted >= 0),
    cursor_checkpoint JSONB,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ingest_runs_finished_after_start
        CHECK (finished_at IS NULL OR finished_at >= started_at)
);

COMMENT ON TABLE ingest_runs IS 'One row per source fetch execution.';
COMMENT ON COLUMN ingest_runs.cursor_checkpoint IS 'Source-specific cursor/checkpoint blob used for incremental fetch.';
COMMENT ON COLUMN ingest_runs.triggered_by_topic IS 'Legacy trace only: topic cron that triggered this source run during migration.';

CREATE INDEX idx_ingest_runs_source_started_at ON ingest_runs (source, started_at DESC);
CREATE INDEX idx_ingest_runs_status_started_at ON ingest_runs (status, started_at DESC);

CREATE TABLE source_checkpoints (
    source TEXT PRIMARY KEY CHECK (length(trim(source)) > 0),
    cursor_checkpoint JSONB NOT NULL,
    last_successful_run_id BIGINT REFERENCES ingest_runs(run_id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE source_checkpoints IS 'Latest committed checkpoint per source for incremental ingestion.';

CREATE TABLE raw_arrivals (
    raw_arrival_id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES ingest_runs(run_id) ON DELETE RESTRICT,
    source TEXT NOT NULL CHECK (length(trim(source)) > 0),
    source_item_id TEXT NOT NULL CHECK (length(trim(source_item_id)) > 0),
    source_item_url TEXT,
    source_published_at TIMESTAMPTZ,
    fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    raw_payload JSONB NOT NULL,
    payload_hash BYTEA NOT NULL CHECK (octet_length(payload_hash) = 32),
    response_headers JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_raw_arrivals_source_item_payload UNIQUE (source, source_item_id, payload_hash)
);

COMMENT ON TABLE raw_arrivals IS 'Append-only immutable ledger of fetched source items.';
COMMENT ON COLUMN raw_arrivals.payload_hash IS 'SHA-256 of canonicalized payload JSON for idempotent insert.';

CREATE INDEX idx_raw_arrivals_source_item_fetched ON raw_arrivals (source, source_item_id, fetched_at DESC);
CREATE INDEX idx_raw_arrivals_fetched_at ON raw_arrivals (fetched_at DESC);
CREATE INDEX idx_raw_arrivals_payload_hash ON raw_arrivals (payload_hash);
CREATE INDEX idx_raw_arrivals_payload_gin ON raw_arrivals USING gin (raw_payload jsonb_path_ops);

CREATE TABLE documents (
    document_id BIGSERIAL PRIMARY KEY,
    raw_arrival_id BIGINT NOT NULL UNIQUE REFERENCES raw_arrivals(raw_arrival_id) ON DELETE CASCADE,
    source TEXT NOT NULL CHECK (length(trim(source)) > 0),
    source_item_id TEXT NOT NULL CHECK (length(trim(source_item_id)) > 0),
    canonical_url TEXT,
    canonical_url_hash BYTEA CHECK (canonical_url_hash IS NULL OR octet_length(canonical_url_hash) = 32),
    normalized_title TEXT NOT NULL CHECK (length(trim(normalized_title)) > 0),
    normalized_text TEXT NOT NULL DEFAULT '',
    normalized_language TEXT NOT NULL DEFAULT 'und',
    published_at TIMESTAMPTZ,
    source_domain TEXT,
    title_simhash BIGINT,
    text_simhash BIGINT,
    title_hash BYTEA CHECK (title_hash IS NULL OR octet_length(title_hash) = 32),
    content_hash BYTEA NOT NULL CHECK (octet_length(content_hash) = 32),
    token_count INTEGER NOT NULL DEFAULT 0 CHECK (token_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE documents IS 'Normalized canonical documents derived from raw arrivals.';
COMMENT ON COLUMN documents.canonical_url_hash IS 'SHA-256 of canonical_url after URL normalization.';
COMMENT ON COLUMN documents.content_hash IS 'SHA-256 of normalized title + normalized text.';

CREATE INDEX idx_documents_source_item ON documents (source, source_item_id);
CREATE INDEX idx_documents_canonical_url_hash ON documents (canonical_url_hash) WHERE canonical_url_hash IS NOT NULL;
CREATE INDEX idx_documents_content_hash ON documents (content_hash);
CREATE INDEX idx_documents_source_domain_published ON documents (source_domain, published_at DESC);
CREATE INDEX idx_documents_created_at ON documents (created_at DESC);
CREATE INDEX idx_documents_title_simhash ON documents (title_simhash) WHERE title_simhash IS NOT NULL;

CREATE TABLE document_embeddings (
    embedding_id BIGSERIAL PRIMARY KEY,
    document_id BIGINT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
    model_name TEXT NOT NULL CHECK (length(trim(model_name)) > 0),
    model_version TEXT NOT NULL CHECK (length(trim(model_version)) > 0),
    embedding vector(4096) NOT NULL,
    embedded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    service_endpoint TEXT NOT NULL DEFAULT 'http://127.0.0.1:8844',
    latency_ms INTEGER CHECK (latency_ms IS NULL OR latency_ms >= 0),
    CONSTRAINT uq_document_embeddings_doc_model UNIQUE (document_id, model_name, model_version)
);

COMMENT ON TABLE document_embeddings IS 'Embedding vectors per document and embedding model/version.';
COMMENT ON COLUMN document_embeddings.embedding IS 'Qwen3-Embedding-8B vector (4096 dimensions).';

CREATE INDEX idx_document_embeddings_document ON document_embeddings (document_id);
CREATE INDEX idx_document_embeddings_model_time ON document_embeddings (model_name, model_version, embedded_at DESC);
CREATE INDEX idx_document_embeddings_embedded_at ON document_embeddings (embedded_at DESC);
CREATE INDEX idx_document_embeddings_hnsw
    ON document_embeddings USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 128);

CREATE TABLE stories (
    story_id BIGSERIAL PRIMARY KEY,
    canonical_title TEXT NOT NULL CHECK (length(trim(canonical_title)) > 0),
    canonical_url TEXT,
    canonical_url_hash BYTEA CHECK (canonical_url_hash IS NULL OR octet_length(canonical_url_hash) = 32),
    representative_document_id BIGINT REFERENCES documents(document_id) ON DELETE SET NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    source_count INTEGER NOT NULL DEFAULT 1 CHECK (source_count >= 1),
    item_count INTEGER NOT NULL DEFAULT 1 CHECK (item_count >= 1),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suppressed', 'merged')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT stories_seen_window_valid CHECK (last_seen_at >= first_seen_at)
);

COMMENT ON TABLE stories IS 'Canonical deduplicated story clusters.';
COMMENT ON COLUMN stories.representative_document_id IS 'Best document used for title/url when presenting the story.';

CREATE INDEX idx_stories_first_seen_at ON stories (first_seen_at DESC);
CREATE INDEX idx_stories_last_seen_at ON stories (last_seen_at DESC);
CREATE INDEX idx_stories_canonical_url_hash ON stories (canonical_url_hash) WHERE canonical_url_hash IS NOT NULL;

CREATE TABLE story_members (
    story_id BIGINT NOT NULL REFERENCES stories(story_id) ON DELETE CASCADE,
    document_id BIGINT NOT NULL UNIQUE REFERENCES documents(document_id) ON DELETE CASCADE,
    match_type story_match_type NOT NULL,
    match_score DOUBLE PRECISION,
    match_details JSONB,
    matched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (story_id, document_id),
    CONSTRAINT story_members_score_range CHECK (match_score IS NULL OR (match_score >= 0 AND match_score <= 1))
);

COMMENT ON TABLE story_members IS 'Membership mapping from normalized document to canonical story.';
COMMENT ON COLUMN story_members.match_details IS 'Structured evidence for why merge occurred (signals and weights).';

CREATE INDEX idx_story_members_story ON story_members (story_id, matched_at DESC);
CREATE INDEX idx_story_members_match_type ON story_members (match_type);

CREATE TABLE dedup_events (
    dedup_event_id BIGSERIAL PRIMARY KEY,
    document_id BIGINT NOT NULL UNIQUE REFERENCES documents(document_id) ON DELETE CASCADE,
    decision dedup_decision NOT NULL,
    chosen_story_id BIGINT REFERENCES stories(story_id) ON DELETE SET NULL,
    best_candidate_story_id BIGINT REFERENCES stories(story_id) ON DELETE SET NULL,
    best_cosine DOUBLE PRECISION,
    title_overlap DOUBLE PRECISION,
    entity_date_consistency DOUBLE PRECISION,
    composite_score DOUBLE PRECISION,
    exact_signal TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT dedup_events_best_cosine_range CHECK (best_cosine IS NULL OR (best_cosine >= 0 AND best_cosine <= 1)),
    CONSTRAINT dedup_events_title_overlap_range CHECK (title_overlap IS NULL OR (title_overlap >= 0 AND title_overlap <= 1)),
    CONSTRAINT dedup_events_entity_date_range CHECK (entity_date_consistency IS NULL OR (entity_date_consistency >= 0 AND entity_date_consistency <= 1)),
    CONSTRAINT dedup_events_composite_range CHECK (composite_score IS NULL OR (composite_score >= 0 AND composite_score <= 1))
);

COMMENT ON TABLE dedup_events IS 'Audit trail of dedup decisions for each processed document.';

CREATE INDEX idx_dedup_events_decision_time ON dedup_events (decision, created_at DESC);
CREATE INDEX idx_dedup_events_chosen_story ON dedup_events (chosen_story_id);

CREATE TABLE topics (
    topic_id SERIAL PRIMARY KEY,
    topic_slug TEXT NOT NULL UNIQUE CHECK (topic_slug ~ '^[a-z0-9_]+$'),
    topic_name TEXT NOT NULL CHECK (length(trim(topic_name)) > 0),
    timezone TEXT NOT NULL DEFAULT 'Europe/Berlin',
    digest_cron TEXT NOT NULL,
    discord_channel_id TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE topics IS 'Digest topic configuration and publish target.';
COMMENT ON COLUMN topics.digest_cron IS 'Cron expression in topic local timezone for digest scheduling.';

CREATE INDEX idx_topics_enabled ON topics (enabled);

CREATE TABLE topic_source_rules (
    topic_id INTEGER NOT NULL REFERENCES topics(topic_id) ON DELETE CASCADE,
    source TEXT NOT NULL CHECK (length(trim(source)) > 0),
    rule_type topic_rule_type NOT NULL DEFAULT 'include',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (topic_id, source, rule_type)
);

COMMENT ON TABLE topic_source_rules IS 'Per-topic source include/exclude rules.';

CREATE INDEX idx_topic_source_rules_topic ON topic_source_rules (topic_id, rule_type);

CREATE TABLE topic_keyword_rules (
    rule_id BIGSERIAL PRIMARY KEY,
    topic_id INTEGER NOT NULL REFERENCES topics(topic_id) ON DELETE CASCADE,
    rule_type topic_rule_type NOT NULL,
    pattern TEXT NOT NULL CHECK (length(trim(pattern)) > 0),
    is_regex BOOLEAN NOT NULL DEFAULT false,
    weight SMALLINT NOT NULL DEFAULT 1,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_topic_keyword_rule UNIQUE (topic_id, rule_type, pattern)
);

COMMENT ON TABLE topic_keyword_rules IS 'Keyword/regex include/exclude topic assignment rules.';

CREATE INDEX idx_topic_keyword_rules_topic_enabled ON topic_keyword_rules (topic_id, enabled);

CREATE TABLE story_topic_state (
    story_id BIGINT NOT NULL REFERENCES stories(story_id) ON DELETE CASCADE,
    topic_id INTEGER NOT NULL REFERENCES topics(topic_id) ON DELETE CASCADE,
    first_seen_in_topic_at TIMESTAMPTZ NOT NULL,
    first_published_at TIMESTAMPTZ,
    last_published_at TIMESTAMPTZ,
    publish_count INTEGER NOT NULL DEFAULT 0 CHECK (publish_count >= 0),
    suppressed BOOLEAN NOT NULL DEFAULT false,
    suppression_reason TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (story_id, topic_id),
    CONSTRAINT story_topic_state_first_pub_after_seen
        CHECK (first_published_at IS NULL OR first_published_at >= first_seen_in_topic_at),
    CONSTRAINT story_topic_state_last_pub_after_first
        CHECK (last_published_at IS NULL OR (first_published_at IS NOT NULL AND last_published_at >= first_published_at))
);

COMMENT ON TABLE story_topic_state IS 'Per-topic novelty and publish state for each canonical story.';

CREATE INDEX idx_story_topic_state_topic_seen ON story_topic_state (topic_id, first_seen_in_topic_at DESC);
CREATE INDEX idx_story_topic_state_topic_first_published ON story_topic_state (topic_id, first_published_at);
CREATE INDEX idx_story_topic_state_topic_suppressed ON story_topic_state (topic_id, suppressed);

CREATE TABLE digest_runs (
    digest_run_id BIGSERIAL PRIMARY KEY,
    topic_id INTEGER NOT NULL REFERENCES topics(topic_id) ON DELETE RESTRICT,
    run_date DATE NOT NULL,
    window_start_utc TIMESTAMPTZ NOT NULL,
    window_end_utc TIMESTAMPTZ NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    status digest_run_status NOT NULL DEFAULT 'running',
    candidate_count INTEGER NOT NULL DEFAULT 0 CHECK (candidate_count >= 0),
    posted_count INTEGER NOT NULL DEFAULT 0 CHECK (posted_count >= 0),
    discord_message_id TEXT,
    error_message TEXT,
    CONSTRAINT uq_digest_runs_topic_date UNIQUE (topic_id, run_date),
    CONSTRAINT digest_runs_window_valid CHECK (window_end_utc > window_start_utc),
    CONSTRAINT digest_runs_finished_after_start CHECK (finished_at IS NULL OR finished_at >= started_at)
);

COMMENT ON TABLE digest_runs IS 'One digest generation/publish run per topic/day.';

CREATE INDEX idx_digest_runs_topic_date ON digest_runs (topic_id, run_date DESC);
CREATE INDEX idx_digest_runs_status_started ON digest_runs (status, started_at DESC);

CREATE TABLE digest_entries (
    digest_entry_id BIGSERIAL PRIMARY KEY,
    digest_run_id BIGINT NOT NULL REFERENCES digest_runs(digest_run_id) ON DELETE CASCADE,
    story_id BIGINT NOT NULL REFERENCES stories(story_id) ON DELETE RESTRICT,
    status digest_entry_status NOT NULL,
    rank INTEGER,
    reason TEXT,
    score DOUBLE PRECISION,
    discord_message_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_digest_entries_run_story UNIQUE (digest_run_id, story_id),
    CONSTRAINT digest_entries_rank_positive CHECK (rank IS NULL OR rank > 0),
    CONSTRAINT digest_entries_score_range CHECK (score IS NULL OR (score >= 0 AND score <= 1))
);

COMMENT ON TABLE digest_entries IS 'Per-story decision for each digest run (included or excluded).' ;

CREATE INDEX idx_digest_entries_run_status_rank ON digest_entries (digest_run_id, status, rank);
CREATE INDEX idx_digest_entries_story ON digest_entries (story_id);

CREATE TRIGGER trg_ingest_runs_touch_updated_at
BEFORE UPDATE ON ingest_runs
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TRIGGER trg_documents_touch_updated_at
BEFORE UPDATE ON documents
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TRIGGER trg_stories_touch_updated_at
BEFORE UPDATE ON stories
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TRIGGER trg_topics_touch_updated_at
BEFORE UPDATE ON topics
FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

INSERT INTO topics (topic_slug, topic_name, timezone, digest_cron, discord_channel_id, enabled)
VALUES
    ('ai_news', 'AI News', 'Europe/Berlin', '0 9 * * *', 'REPLACE_WITH_AI_CHANNEL_ID', true),
    ('world_news', 'World News', 'Europe/Berlin', '0 9 * * *', 'REPLACE_WITH_WORLD_CHANNEL_ID', true),
    ('china_news', 'China News', 'Europe/Berlin', '0 10 * * *', 'REPLACE_WITH_CHINA_CHANNEL_ID', true)
ON CONFLICT (topic_slug) DO NOTHING;

COMMIT;
```

## 2. Phase-by-Phase Migration Plan (No Cron Breakage)

Current production cron jobs must keep posting during transition:
- `AI News` at `09:00 Europe/Berlin`
- `World News` at `09:00 Europe/Berlin`
- `China News` at `10:00 Europe/Berlin`

### Phase 0 - Freeze Interfaces and Add Flags (0.5 day)

Goal: make migration safe and reversible.

Tasks:
- Add runtime mode flags:
  - `NP_MODE=legacy|shadow|prod`
  - `NP_PUBLISH_V2=false|true`
  - `NP_WRITE_RAW_LEDGER=false|true`
- Snapshot current behavior for comparison:
  - Last 7 days posted digests per topic.
  - Current JSONL dedup files.
- Define fallback contract:
  - If v2 digest run fails before post deadline, keep current legacy script posting.

Rollback: no-op (all legacy paths unchanged).

### Phase 1 - Database Bootstrap (1 day)

Goal: create schema and observability without changing digest output.

Tasks:
- Deploy PostgreSQL 16 + `pgvector`.
- Apply migration `001_initial_schema.sql`.
- Configure backups (`pg_dump` nightly for v1).
- Add basic SQL health checks:
  - write/read latency
  - row growth (`raw_arrivals`, `documents`, `stories`)

Cron impact: none. Legacy jobs remain source of truth.

Rollback: keep DB idle; legacy continues.

### Phase 2 - Dual-Write Ingestion from Existing Jobs (1 day)

Goal: preserve current fetch behavior but start writing ledger.

Tasks:
- Wrap existing source fetchers so each run:
  - starts `ingest_runs`
  - inserts `raw_arrivals`
  - updates `source_checkpoints`
- Keep legacy JSONL dedup path untouched.
- Ensure idempotency with `ON CONFLICT` against `(source, source_item_id, payload_hash)`.

Cron impact:
- Existing 3 topic jobs unchanged.
- They now also dual-write raw items to DB.

Rollback:
- Set `NP_WRITE_RAW_LEDGER=false`; legacy behavior fully preserved.

### Phase 3 - Normalize + Embed + Dedup in Shadow (1.5 days)

Goal: build canonical stories without changing published digests.

Tasks:
- Add workers for:
  - normalize pending `raw_arrivals -> documents`
  - embed pending `documents -> document_embeddings` via `:8844`
  - dedup pending `documents -> stories/story_members/dedup_events`
- Run workers every 10-15 minutes (or as a single loop daemon).
- Populate `story_topic_state` in shadow mode.
- Store gray-zone decisions in `dedup_events` only; do not suppress from legacy digest.

Cron impact: existing digests still posted by legacy scripts.

Rollback:
- Stop workers; ledger stays intact.

### Phase 4 - Shadow Digest Generation and Diffing (1 day)

Goal: prove v2 digest quality before cutover.

Tasks:
- Run v2 digest builder at same schedule as production, but `NP_PUBLISH_V2=false`.
- Persist `digest_runs` + `digest_entries` with `status=completed` in shadow.
- Compare shadow vs legacy for 3 consecutive days:
  - overlap rate
  - unexpected misses
  - duplicate suppression improvements
- Manually review at least 30 gray-zone pairs.

Cron impact: no user-visible change.

Rollback: none needed.

### Phase 5 - Controlled Cutover by Topic (1 day)

Goal: switch posting to canonical stories, one topic at a time.

Tasks:
- Day 1 cutover `AI News` (09:00) to v2 publisher.
- Keep `World` and `China` on legacy for one more day.
- Day 2 cutover `World` + `China`.
- Keep legacy scripts callable as automatic fallback for 7 days.

Cron impact:
- Same schedule, different command target for each topic.

Rollback:
- Flip `NP_PUBLISH_V2=false` per topic and execute legacy publisher.

### Phase 6 - Decommission Legacy JSONL Dedup (0.5 day)

Goal: remove split-brain state.

Tasks:
- Freeze JSONL dedup files as read-only archive.
- Remove legacy dedup write path from cron scripts.
- Keep a one-time backfill utility for historical import if needed.

Rollback:
- Use archived JSONL + legacy publisher for emergency only.

## 3. Go Worker Architecture

### 3.1 Repository Structure

```text
scoop/
  go.mod
  go.sum
  cmd/
    scoop/
      main.go
  internal/
    config/
      config.go
    logging/
      logger.go
    db/
      pool.go
      tx.go
    sql/
      ingest.sql
      normalize.sql
      embed.sql
      dedup.sql
      topics.sql
      digest.sql
    fetchers/
      interfaces.go
      hackernews.go
      arxiv.go
      reddit.go
      youtube.go
      rss.go               # SCMP, Nikkei, AI lab blogs, tech press
    normalize/
      url.go
      text.go
      hashes.go
      entities.go
      worker.go
    embed/
      client.go            # HTTP client for :8844
      worker.go
    dedup/
      exact.go
      lexical.go
      semantic.go
      scorer.go
      worker.go
    topics/
      rules.go
      worker.go
    digest/
      query.go
      render.go
      publish_openclaw.go
      worker.go
    app/
      run_once.go
      commands.go
  migrations/
    001_initial_schema.sql
  tests/
    integration_ingest_test.go
    dedup_thresholds_test.go
    digest_query_test.go
```

### 3.2 Entry Points

Expose a single Go CLI binary with subcommands:

- `scoop ingest --source <name|all>`
- `scoop normalize --limit 1000`
- `scoop embed --model Qwen3-Embedding-8B --model-version v1 --batch-size 32`
- `scoop dedup --lookback-days 365 --limit 2000`
- `scoop assign-topics --limit 2000`
- `scoop digest --topic <slug> --date YYYY-MM-DD [--publish]`
- `scoop run-once --publish` (orchestrates all stages in order)

### 3.3 How Workers Connect (Data Contracts)

1. `ingest` worker
- Input: source APIs/feeds.
- Output: `ingest_runs`, `raw_arrivals`, `source_checkpoints`.

2. `normalize` worker
- Claims `raw_arrivals` not present in `documents`.
- Produces canonical URL/text/hashes into `documents`.

3. `embed` worker
- Claims `documents` missing embedding for current `(model_name, model_version)`.
- Calls `POST http://127.0.0.1:8844/...` in batches.
- Writes `document_embeddings`.

4. `dedup` worker
- Claims embedded documents missing `story_members`.
- Runs exact -> lexical -> semantic rules.
- Writes `stories`, `story_members`, `dedup_events`.

5. `assign-topics` worker
- For new stories or changed topic rules, updates `story_topic_state`.

6. `digest` worker
- Builds daily candidate set per topic.
- Inserts `digest_runs` + `digest_entries`.
- If `--publish`, posts via existing OpenClaw sub-agent session flow and stores `discord_message_id`.
- Updates `story_topic_state.first_published_at/last_published_at/publish_count` only after successful publish.

### 3.4 Runtime Guarantees

- Idempotency: every stage uses `INSERT ... ON CONFLICT`.
- Concurrency control:
  - stage-level `pg_try_advisory_lock` (avoid duplicate worker loops)
  - row-level `FOR UPDATE SKIP LOCKED` for batch claims.
- Failure semantics:
  - partial failures stay retryable.
  - no hard deletes from ledger tables.
- Operational behavior:
  - one static binary deploy target (`scoop`) for cron and manual runs.
  - graceful shutdown via context cancellation so in-flight batches can commit/rollback cleanly.

## 4. Threshold Recommendations (with Rationale)

Current cosine threshold `0.82` is too low for safe auto-merge in mixed-source news; it will merge related but distinct stories.

Recommended production defaults:

| Stage | Rule | Threshold | Action | Rationale |
|---|---|---:|---|---|
| Exact | Canonical URL hash match | exact | Auto-merge | Deterministic identity after URL normalization |
| Exact | Source-native ID match (arXiv id, Reddit submission id, YouTube video id, HN item+outbound URL) | exact | Auto-merge | Deterministic per source |
| Exact | Content hash match | exact | Auto-merge | Same normalized text payload |
| Lexical | Title simhash distance | `<= 3` bits | Auto-merge | Captures trivial rewrites/copy edits |
| Lexical | Title trigram Jaccard | `>= 0.88` | Auto-merge if date delta `<= 14d` | Prevents old-event collisions |
| Semantic | Cosine high-confidence | `>= 0.935` + title overlap `>= 0.30` | Auto-merge | High precision in multi-source news |
| Semantic | Very high cosine override | `>= 0.965` | Auto-merge | Allows sparse-title matches |
| Semantic | Gray zone | `0.89 - 0.935` | Mark `possible_duplicate`; do not suppress automatically | Avoid false merges |
| Semantic | New story | `< 0.89` | Create new story | Conservative default |

Composite score recommendation (for diagnostics, not sole gate):

`composite = 0.75 * cosine + 0.15 * title_overlap + 0.10 * entity_date_consistency`

Model/search settings:
- Embedding model: `Qwen3-Embedding-8B`, version pinned in DB.
- ANN index: HNSW cosine (`m=16`, `ef_construction=128`).
- Retrieval: top `K=20` candidates, lookback `365` days.
- Runtime search `ef_search`: start `64`, raise to `100` if recall misses appear.

Calibration plan (first week after cutover):
- Sample 50 auto-merges + 50 gray-zone pairs.
- Measure false-merge rate.
- Adjust only one threshold at a time.

## 5. Estimated Effort by Phase

Assumes 1 engineer familiar with existing OpenClaw jobs.

| Phase | Effort | Output |
|---|---:|---|
| Phase 0 - Flags and baseline snapshots | 0.5 day | Safe toggles + comparison corpus |
| Phase 1 - DB bootstrap and migration | 1.0 day | Running Postgres+pgvector schema |
| Phase 2 - Dual-write ingestion | 1.0 day | Ledger population without behavior change |
| Phase 3 - Normalize/embed/dedup shadow | 1.5 days | Canonical stories and topic state |
| Phase 4 - Shadow digest + diffing | 1.0 day | Quality validation on real traffic |
| Phase 5 - Incremental cutover | 1.0 day | v2 live for all topics |
| Phase 6 - Legacy dedup retirement | 0.5 day | Single source of truth |

Total: ~6.5 engineering days (calendar: 5-8 days including observation windows).

## 6. Concrete v1 That Ships in Days

### 6.1 v1 Scope (Ship Fast)

Include:
- Sources: HN, arXiv, Reddit, YouTube, RSS feeds (SCMP, Nikkei Asia, AI lab blogs, tech press).
- One Postgres DB with schema above.
- One process/CLI capable of running all workers.
- Daily digest generation for existing 3 topics.
- Discord publishing through existing OpenClaw sub-agent session mechanism.

Defer (not in v1):
- Manual review UI.
- LLM adjudicator for gray-zone pairs.
- Advanced topic classifier training.
- Cross-region multi-node scaling.

### 6.2 5-Day Execution Plan

Day 1:
- Stand up DB + migration.
- Initialize Go module and CLI command skeleton.
- Add dual-write ledger inserts to existing fetch pipeline.

Day 2:
- Implement normalization and embedding workers.
- Validate embedding throughput on `:8844` (batch 32; tune to 64 only if stable).

Day 3:
- Implement dedup worker + topic assignment.
- Start continuous shadow processing.

Day 4:
- Implement digest worker + OpenClaw publisher adapter.
- Run shadow digests at production times; compare with legacy output.

Day 5:
- Cutover AI topic first, observe.
- Cutover World + China after one successful cycle.
- Keep automatic fallback to legacy for 7 days.

### 6.3 Cron Transition (Concrete)

During migration:
- Keep current jobs exactly as scheduled.
- Add worker loop cron every 15 minutes:
  - `*/15 * * * * scoop run-once`
- Add shadow digest jobs at current publish times with `--publish` disabled.

After cutover:
- Keep same topic cron times:
  - `0 9 * * *` -> `scoop digest --topic ai_news --date {{today_berlin}} --publish`
  - `0 9 * * *` -> `scoop digest --topic world_news --date {{today_berlin}} --publish`
  - `0 10 * * *` -> `scoop digest --topic china_news --date {{today_berlin}} --publish`
- Keep ingestion/processing cron independent of topic schedules.

### 6.4 Acceptance Checklist for v1

- [ ] Raw arrivals persist for all sources with no data loss.
- [ ] `documents` coverage >= 99% of raw arrivals.
- [ ] Embeddings generated for >= 99% of documents.
- [ ] Duplicate rate in posted digest reduced vs legacy baseline.
- [ ] No repeated `story_id` in same topic digest/day.
- [ ] Fallback to legacy can be executed within 5 minutes.

## 7. Canonical News Item Schema Standard

Decision for v1:
- Use an internal canonical schema for ingestion/normalization.
- Base field semantics on `schema.org/NewsArticle`.
- Enforce ingest validation with JSON Schema Draft 2020-12.
- Keep Dublin Core as optional mapping/import-export only.
- Use explicit ID handles in metadata for traceability:
  - `source_metadata.scrape_run_uuid` (one UUID per scrape execution)
  - `source_metadata.item_uuid` (stable UUID per source item, deterministic UUIDv5 recommended)
  - `story_uuid` for canonical deduped stories (planned; current DB handle is `story_id`)

Source of truth:
- `NEWS_ITEM_SCHEMA.md`

Why:
- Dublin Core alone is too minimal for robust dedup and ranking.
- `NewsArticle` semantics map well to modern web/news metadata.
- Internal schema + validation prevents source drift and keeps pipeline behavior predictable.
