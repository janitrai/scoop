#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env"
DB_NAME="${DB_NAME:-scoop}"
DATASET_DIR="$ROOT_DIR/testdata/scraped_news"
GROUND_TRUTH_FILE="$ROOT_DIR/testdata/ground_truth/dedup_ground_truth_items.jsonl"
MODEL_NAME="gt-eval-mock"
MODEL_VERSION="v1"
LOOKBACK_DAYS=365
EMBED_BATCH_SIZE=64
LIMIT=100000
EMBED_ENDPOINT="http://127.0.0.1:18846/embed"
MIN_F1=0.99
KEEP_DATA=0

BIN_PATH="${TMPDIR:-/tmp}/scoop-gt-eval"
MOCK_SRC=""
MOCK_LOG=""
MOCK_PID=""
MAP_TSV=""
PRED_TSV=""

usage() {
  cat <<'EOF'
Usage: scripts/eval_dedup_ground_truth.sh [options]

Options:
  --dataset-dir <path>        Directory with scraped JSON fixtures
  --ground-truth <path>       Ground-truth JSONL annotation file
  --env-file <path>           Path to .env
  --lookback-days <N>         Dedup lookback window in days (default: 365)
  --embed-batch-size <N>      Embedding batch size (default: 64)
  --limit <N>                 Normalize/embed/dedup limit (default: 100000)
  --min-f1 <float>            Optional fail threshold for pairwise F1
  --keep-data                 Keep DB data after evaluation
  -h, --help                  Show help
EOF
}

log() {
  printf '[%s] %s\n' "$(date -Iseconds)" "$*"
}

reset_pipeline_tables() {
  PGPASSWORD="$PGPASSWORD" psql -X -v ON_ERROR_STOP=1 -h localhost -U news -d "$DB_NAME" <<'SQL'
TRUNCATE TABLE
  news.digest_entries,
  news.digest_runs,
  news.story_topic_state,
  news.dedup_events,
  news.story_articles,
  news.stories,
  news.article_embeddings,
  news.articles,
  news.raw_arrivals,
  news.source_checkpoints,
  news.ingest_runs
RESTART IDENTITY CASCADE;
SQL
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
  if [[ -n "$MAP_TSV" ]]; then
    rm -f "$MAP_TSV"
  fi
  if [[ -n "$PRED_TSV" ]]; then
    rm -f "$PRED_TSV"
  fi
  if [[ "$KEEP_DATA" -eq 0 ]]; then
    reset_pipeline_tables >/dev/null 2>&1 || true
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dataset-dir)
      DATASET_DIR="$2"
      shift 2
      ;;
    --ground-truth)
      GROUND_TRUTH_FILE="$2"
      shift 2
      ;;
    --env-file)
      ENV_FILE="$2"
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
    --min-f1)
      MIN_F1="$2"
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
if [[ ! -f "$GROUND_TRUTH_FILE" ]]; then
  echo "Missing ground truth file: $GROUND_TRUTH_FILE" >&2
  exit 1
fi

# shellcheck disable=SC1090
source "$ENV_FILE"
if [[ -z "${PGPASSWORD:-}" ]]; then
  echo "PGPASSWORD is required in $ENV_FILE" >&2
  exit 1
fi

files_total=$(find "$DATASET_DIR" -type f -name '*.json' | wc -l | tr -d ' ')
if [[ "$files_total" -eq 0 ]]; then
  echo "No JSON files found under $DATASET_DIR" >&2
  exit 1
fi

log "Building evaluation binary"
go build -o "$BIN_PATH" "$ROOT_DIR/cmd/scoop"

trap cleanup EXIT

log "Resetting pipeline tables before evaluation"
reset_pipeline_tables >/dev/null

MOCK_SRC="$(mktemp /tmp/scoop-gt-eval-mock-XXXX.go)"
MOCK_LOG="$(mktemp /tmp/scoop-gt-eval-mock-XXXX.log)"
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

	srv := &http.Server{Addr: "127.0.0.1:18846", Handler: mux}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
GO

log "Starting deterministic mock embedding endpoint at $EMBED_ENDPOINT"
go run "$MOCK_SRC" >"$MOCK_LOG" 2>&1 &
MOCK_PID=$!
sleep 1
if ! kill -0 "$MOCK_PID" >/dev/null 2>&1; then
  echo "Failed to start mock embedding service" >&2
  cat "$MOCK_LOG" >&2 || true
  exit 1
fi

MAP_TSV="$(mktemp /tmp/scoop-gt-map-XXXX.tsv)"
ingest_failed=0
log "Ingesting $files_total files and capturing raw_arrival mapping"
ingest_start_ms=$(date +%s%3N)
while IFS= read -r -d '' file; do
  rel_file="$file"
  if [[ "$rel_file" == "$ROOT_DIR/"* ]]; then
    rel_file="${rel_file#"$ROOT_DIR"/}"
  fi

  output="$("$BIN_PATH" ingest --env "$ENV_FILE" --payload-file "$file" --triggered-by-topic gt_eval 2>&1 || true)"
  inserted="$(printf '%s\n' "$output" | sed -n 's/^run_id=.*inserted=\(true\|false\).*/\1/p' | tail -n1)"
  raw_arrival_id="$(printf '%s\n' "$output" | sed -n 's/^raw_arrival_id=\([0-9][0-9]*\)$/\1/p' | tail -n1)"

  if [[ "$inserted" != "true" || -z "$raw_arrival_id" ]]; then
    ingest_failed=$((ingest_failed + 1))
    printf 'INGEST_FAILED\t%s\t%s\n' "$rel_file" "$output" >&2
    continue
  fi

  printf '%s\t%s\n' "$raw_arrival_id" "$rel_file" >> "$MAP_TSV"
done < <(find "$DATASET_DIR" -type f -name '*.json' -print0 | sort -z)
ingest_end_ms=$(date +%s%3N)
ingest_ms=$((ingest_end_ms - ingest_start_ms))
map_lines=$(wc -l < "$MAP_TSV" | tr -d ' ')

if [[ "$ingest_failed" -gt 0 ]]; then
  echo "Ingest failures encountered: $ingest_failed" >&2
  exit 1
fi

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

log "Running dedup"
dedup_start_ms=$(date +%s%3N)
dedup_output="$("$BIN_PATH" dedup --env "$ENV_FILE" --limit "$LIMIT" --lookback-days "$LOOKBACK_DAYS" --model-name "$MODEL_NAME" --model-version "$MODEL_VERSION" 2>&1)"
dedup_end_ms=$(date +%s%3N)
dedup_ms=$((dedup_end_ms - dedup_start_ms))

PRED_TSV="$(mktemp /tmp/scoop-gt-pred-XXXX.tsv)"
PGPASSWORD="$PGPASSWORD" psql -X -h localhost -U news -d "$DB_NAME" -At -F $'\t' <<'SQL' > "$PRED_TSV"
SELECT d.raw_arrival_id, sm.story_id
FROM news.articles d
JOIN news.story_articles sm ON sm.article_id = d.article_id
ORDER BY d.raw_arrival_id;
SQL
pred_lines=$(wc -l < "$PRED_TSV" | tr -d ' ')

eval_output="$(
python3 - "$GROUND_TRUTH_FILE" "$MAP_TSV" "$PRED_TSV" "$MIN_F1" <<'PY'
import json,sys,itertools

gt_path,map_path,pred_path,min_f1=sys.argv[1],sys.argv[2],sys.argv[3],float(sys.argv[4])

gt={}
with open(gt_path) as fh:
    for line in fh:
        rec=json.loads(line)
        gt[rec['file']]=rec['story_gt_id']

raw_to_file={}
with open(map_path) as fh:
    for line in fh:
        line=line.rstrip('\n')
        if not line:
            continue
        parts=line.split('\t',1)
        if len(parts)!=2:
            continue
        raw_id,file=parts
        raw_to_file[int(raw_id)]=file

pred={}
with open(pred_path) as fh:
    for line in fh:
        line=line.rstrip('\n')
        if not line:
            continue
        parts=line.split('\t',1)
        if len(parts)!=2:
            continue
        raw_id,story_id=parts
        pred[int(raw_id)]=story_id

print(f'eval_mapping_rows={len(raw_to_file)}')
print(f'eval_predicted_rows={len(pred)}')

records=[]
missing_gt=0
missing_pred=0
for raw_id,file in sorted(raw_to_file.items()):
    gt_id=gt.get(file)
    if gt_id is None:
        missing_gt+=1
        continue
    pred_id=pred.get(raw_id)
    if pred_id is None:
        missing_pred+=1
        continue
    records.append((raw_id,file,gt_id,pred_id))

n=len(records)
tp=fp=fn=tn=0
for i in range(n):
    g1=records[i][2]
    p1=records[i][3]
    for j in range(i+1,n):
        same_gt=(g1==records[j][2])
        same_pred=(p1==records[j][3])
        if same_gt and same_pred:
            tp+=1
        elif (not same_gt) and same_pred:
            fp+=1
        elif same_gt and (not same_pred):
            fn+=1
        else:
            tn+=1

precision=tp/(tp+fp) if (tp+fp)>0 else 0.0
recall=tp/(tp+fn) if (tp+fn)>0 else 0.0
f1=(2*precision*recall/(precision+recall)) if (precision+recall)>0 else 0.0

gt_story_count=len({r[2] for r in records})
pred_story_count=len({r[3] for r in records})

print(f'eval_records={n}')
print(f'eval_missing_gt={missing_gt}')
print(f'eval_missing_pred={missing_pred}')
print(f'eval_gt_story_count={gt_story_count}')
print(f'eval_pred_story_count={pred_story_count}')
print(f'eval_pair_tp={tp}')
print(f'eval_pair_fp={fp}')
print(f'eval_pair_fn={fn}')
print(f'eval_pair_tn={tn}')
print(f'eval_pair_precision={precision:.6f}')
print(f'eval_pair_recall={recall:.6f}')
print(f'eval_pair_f1={f1:.6f}')

if min_f1 > 0 and f1 < min_f1:
    print(f'eval_status=FAIL f1<{min_f1:.6f}', file=sys.stderr)
    sys.exit(2)
print('eval_status=PASS')
PY
)"

echo "Dedup Ground Truth Evaluation"
echo "dataset_dir=$DATASET_DIR"
echo "ground_truth_file=$GROUND_TRUTH_FILE"
echo "files_total=$files_total"
echo "files_ingest_failed=$ingest_failed"
echo "mapping_rows=$map_lines"
echo "predicted_rows=$pred_lines"
echo "lookback_days=$LOOKBACK_DAYS"
echo "timing_ingest_ms=$ingest_ms"
echo "timing_normalize_ms=$normalize_ms"
echo "timing_embed_ms=$embed_ms"
echo "timing_dedup_ms=$dedup_ms"
echo "normalize_output=$(printf '%s' "$normalize_output" | tail -n1)"
echo "embed_output=$(printf '%s' "$embed_output" | tail -n1)"
echo "dedup_output=$(printf '%s' "$dedup_output" | tail -n1)"
printf '%s\n' "$eval_output"

if [[ "$KEEP_DATA" -eq 0 ]]; then
  log "Evaluation complete (cleanup enabled)"
else
  log "Evaluation complete (data kept in DB)"
fi
