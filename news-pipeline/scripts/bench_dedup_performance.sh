#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env"
DATASET_DIR="$ROOT_DIR/testdata/scraped_news"
MODEL_NAME="perf-mock"
MODEL_VERSION="v1"
LOOKBACK_DAYS=365
EMBED_BATCH_SIZE=64
LIMIT=100000
EMBED_ENDPOINT="http://127.0.0.1:18845/embed"
KEEP_DATA=0

BIN_PATH="${TMPDIR:-/tmp}/news-pipeline-perf"
MOCK_SRC=""
MOCK_LOG=""
MOCK_PID=""

usage() {
  cat <<'EOF'
Usage: scripts/bench_dedup_performance.sh [options]

Options:
  --dataset-dir <path>      Directory with schema-valid item JSON files
  --env-file <path>         Path to .env used by CLI and psql
  --model-name <name>       Embedding model_name key (default: perf-mock)
  --model-version <ver>     Embedding model_version key (default: v1)
  --lookback-days <N>       Dedup candidate lookback window in days (default: 365)
  --embed-batch-size <N>    Batch size for embed stage (default: 64)
  --limit <N>               Stage processing limit (default: 100000)
  --keep-data               Keep benchmark data in DB after run
  -h, --help                Show help
EOF
}

log() {
  printf '[%s] %s\n' "$(date -Iseconds)" "$*"
}

cleanup() {
  if [[ -n "$MOCK_PID" ]]; then
    kill "$MOCK_PID" >/dev/null 2>&1 || true
  fi
  if [[ -n "$MOCK_SRC" ]]; then
    rm -f "$MOCK_SRC"
  fi
  if [[ -n "$MOCK_LOG" ]]; then
    rm -f "$MOCK_LOG"
  fi
  if [[ "$KEEP_DATA" -eq 0 ]]; then
    reset_pipeline_tables >/dev/null 2>&1 || true
  fi
}

reset_pipeline_tables() {
  PGPASSWORD="$PGPASSWORD" psql -X -v ON_ERROR_STOP=1 -h localhost -U news -d news_pipeline <<'SQL'
TRUNCATE TABLE
  news.digest_entries,
  news.digest_runs,
  news.story_topic_state,
  news.dedup_events,
  news.story_members,
  news.stories,
  news.document_embeddings,
  news.documents,
  news.raw_arrivals,
  news.source_checkpoints,
  news.ingest_runs
RESTART IDENTITY CASCADE;
SQL
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dataset-dir)
      DATASET_DIR="$2"
      shift 2
      ;;
    --env-file)
      ENV_FILE="$2"
      shift 2
      ;;
    --model-name)
      MODEL_NAME="$2"
      shift 2
      ;;
    --model-version)
      MODEL_VERSION="$2"
      shift 2
      ;;
    --lookback-days)
      LOOKBACK_DAYS="$2"
      shift 2
      ;;
    --embed-batch-size)
      EMBED_BATCH_SIZE="$2"
      shift 2
      ;;
    --limit)
      LIMIT="$2"
      shift 2
      ;;
    --keep-data)
      KEEP_DATA=1
      shift 1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing env file: $ENV_FILE" >&2
  exit 1
fi
if [[ ! -d "$DATASET_DIR" ]]; then
  echo "Missing dataset dir: $DATASET_DIR" >&2
  exit 1
fi

# shellcheck disable=SC1090
source "$ENV_FILE"
if [[ -z "${PGPASSWORD:-}" ]]; then
  echo "PGPASSWORD is required in $ENV_FILE" >&2
  exit 1
fi

ingest_total=$(find "$DATASET_DIR" -type f -name '*.json' | wc -l | tr -d ' ')
if [[ "$ingest_total" -eq 0 ]]; then
  echo "No JSON files found under $DATASET_DIR" >&2
  exit 1
fi

log "Building benchmark binary"
go build -o "$BIN_PATH" "$ROOT_DIR/cmd/news-pipeline"

trap cleanup EXIT

log "Resetting pipeline tables before benchmark"
reset_pipeline_tables >/dev/null

MOCK_SRC="$(mktemp /tmp/news-pipeline-embed-mock-XXXX.go)"
MOCK_LOG="$(mktemp /tmp/news-pipeline-embed-mock-XXXX.log)"
cat > "$MOCK_SRC" <<'GO'
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"math"
	"net/http"
)

type embedReq struct {
	Texts []string `json:"texts"`
	Input []string `json:"input"`
}

func vectorForText(text string) []float64 {
	h := sha256.Sum256([]byte(text))
	seed := binary.LittleEndian.Uint64(h[:8])
	if seed == 0 {
		seed = 1
	}
	x := seed
	v := make([]float64, 4096)
	var sum float64
	for i := 0; i < 4096; i++ {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		n := int64(x%2000001) - 1000000
		f := float64(n) / 1000000.0
		v[i] = f
		sum += f * f
	}
	norm := math.Sqrt(sum)
	if norm == 0 {
		v[0] = 1
		return v
	}
	for i := range v {
		v[i] /= norm
	}
	return v
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/embed", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		var req embedReq
		_ = json.Unmarshal(body, &req)
		items := req.Texts
		if len(items) == 0 {
			items = req.Input
		}

		embeddings := make([][]float64, len(items))
		for i, text := range items {
			embeddings[i] = vectorForText(text)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": embeddings})
	})

	srv := &http.Server{Addr: "127.0.0.1:18845", Handler: mux}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
GO

log "Starting local mock embedding endpoint at $EMBED_ENDPOINT"
go run "$MOCK_SRC" >"$MOCK_LOG" 2>&1 &
MOCK_PID=$!
sleep 1
if ! kill -0 "$MOCK_PID" >/dev/null 2>&1; then
  echo "Failed to start mock embedding service" >&2
  cat "$MOCK_LOG" >&2 || true
  exit 1
fi

ingest_failed=0
log "Ingesting $ingest_total files from $DATASET_DIR"
ingest_start_ms=$(date +%s%3N)
while IFS= read -r -d '' file; do
  if ! "$BIN_PATH" ingest --env "$ENV_FILE" --payload-file "$file" --triggered-by-topic perf_bench >/dev/null 2>&1; then
    ingest_failed=$((ingest_failed + 1))
  fi
done < <(find "$DATASET_DIR" -type f -name '*.json' -print0 | sort -z)
ingest_end_ms=$(date +%s%3N)
ingest_ms=$((ingest_end_ms - ingest_start_ms))

log "Running normalize"
normalize_start_ms=$(date +%s%3N)
normalize_output="$("$BIN_PATH" normalize --env "$ENV_FILE" --limit "$LIMIT" 2>&1)"
normalize_end_ms=$(date +%s%3N)
normalize_ms=$((normalize_end_ms - normalize_start_ms))

log "Running embed"
embed_start_ms=$(date +%s%3N)
embed_output="$("$BIN_PATH" embed --env "$ENV_FILE" --limit "$LIMIT" --batch-size "$EMBED_BATCH_SIZE" --endpoint "$EMBED_ENDPOINT" --model-name "$MODEL_NAME" --model-version "$MODEL_VERSION" 2>&1)"
embed_end_ms=$(date +%s%3N)
embed_ms=$((embed_end_ms - embed_start_ms))

log "Running dedup (full pass)"
dedup_start_ms=$(date +%s%3N)
dedup_output="$("$BIN_PATH" dedup --env "$ENV_FILE" --limit "$LIMIT" --lookback-days "$LOOKBACK_DAYS" --model-name "$MODEL_NAME" --model-version "$MODEL_VERSION" 2>&1)"
dedup_end_ms=$(date +%s%3N)
dedup_ms=$((dedup_end_ms - dedup_start_ms))

log "Running dedup (empty pass)"
dedup_empty_start_ms=$(date +%s%3N)
dedup_empty_output="$("$BIN_PATH" dedup --env "$ENV_FILE" --limit "$LIMIT" --lookback-days "$LOOKBACK_DAYS" --model-name "$MODEL_NAME" --model-version "$MODEL_VERSION" 2>&1)"
dedup_empty_end_ms=$(date +%s%3N)
dedup_empty_ms=$((dedup_empty_end_ms - dedup_empty_start_ms))

dedup_processed=$(printf '%s\n' "$dedup_output" | sed -n 's/.*dedup processed=\([0-9]*\).*/\1/p' | tail -n1)
if [[ -z "$dedup_processed" ]]; then
  dedup_processed=0
fi

dedup_docs_per_sec="0.00"
if [[ "$dedup_ms" -gt 0 ]]; then
  dedup_docs_per_sec=$(awk -v docs="$dedup_processed" -v ms="$dedup_ms" 'BEGIN { printf "%.2f", (docs * 1000.0) / ms }')
fi

log "Collecting DB metrics"
metrics=$(PGPASSWORD="$PGPASSWORD" psql -X -h localhost -U news -d news_pipeline -At -F '|' <<SQL
SELECT 'raw_arrivals', count(*) FROM news.raw_arrivals
UNION ALL SELECT 'documents', count(*) FROM news.documents
UNION ALL SELECT 'document_embeddings', count(*) FROM news.document_embeddings WHERE model_name='${MODEL_NAME}' AND model_version='${MODEL_VERSION}'
UNION ALL SELECT 'stories', count(*) FROM news.stories
UNION ALL SELECT 'story_members', count(*) FROM news.story_members
UNION ALL SELECT 'dedup_events', count(*) FROM news.dedup_events;
SQL
)

decision_counts=$(PGPASSWORD="$PGPASSWORD" psql -X -h localhost -U news -d news_pipeline -At -F '|' <<'SQL'
SELECT decision, count(*)
FROM news.dedup_events
GROUP BY decision
ORDER BY count(*) DESC;
SQL
)

echo "Dedup Performance Benchmark"
echo "dataset_dir=$DATASET_DIR"
echo "files_total=$ingest_total"
echo "files_ingest_failed=$ingest_failed"
echo "lookback_days=$LOOKBACK_DAYS"
echo "model_name=$MODEL_NAME"
echo "model_version=$MODEL_VERSION"
echo "timing_ingest_ms=$ingest_ms"
echo "timing_normalize_ms=$normalize_ms"
echo "timing_embed_ms=$embed_ms"
echo "timing_dedup_ms=$dedup_ms"
echo "timing_dedup_empty_ms=$dedup_empty_ms"
echo "dedup_processed_docs=$dedup_processed"
echo "dedup_throughput_docs_per_sec=$dedup_docs_per_sec"
echo "normalize_output=$(printf '%s' "$normalize_output" | tail -n1)"
echo "embed_output=$(printf '%s' "$embed_output" | tail -n1)"
echo "dedup_output=$(printf '%s' "$dedup_output" | tail -n1)"
echo "dedup_empty_output=$(printf '%s' "$dedup_empty_output" | tail -n1)"
echo "table_counts:"
printf '%s\n' "$metrics" | sed 's/^/  /'
echo "dedup_decisions:"
if [[ -n "$decision_counts" ]]; then
  printf '%s\n' "$decision_counts" | sed 's/^/  /'
else
  echo "  (none)"
fi

if [[ "$KEEP_DATA" -eq 0 ]]; then
  log "Benchmark complete (cleanup enabled)"
else
  log "Benchmark complete (data kept in DB)"
fi
