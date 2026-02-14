# embedding-service

Production embedding HTTP service for `scoop`.

Contract:
- `POST /embed` with `{"texts":[...],"max_length":512}`
- `POST /v1/embeddings` with `{"input":[...]}` (OpenAI-compatible)
- `GET /health`

The service enforces 4096-dimensional vectors so it matches the current `scoop` pgvector schema.

## Setup

```bash
cd ~/scoop/embedding-service
uv sync
```

`main.py` is the canonical entrypoint. `embed.py` remains as a compatibility wrapper.

## Runtime Modes

1. `transformers` (real model inference)
- Uses local Hugging Face model loading (default `Qwen/Qwen3-Embedding-8B`).
- Requires a working `torch` + `transformers` runtime.

2. `deterministic` (test/dev fallback)
- Fast deterministic vectors.
- No model download required.
- Useful for smoke tests and CI-style checks.

## Quick Checks

```bash
cd ~/scoop/embedding-service
./.venv/bin/python main.py --check
```

## Run Service

Deterministic mode (fast local smoke):

```bash
cd ~/scoop/embedding-service
./.venv/bin/python main.py --server --backend deterministic --host 0.0.0.0 --port 8844
```

Real transformers mode:

```bash
cd ~/scoop/embedding-service
./.venv/bin/python main.py --server --backend transformers --host 0.0.0.0 --port 8844
```

## API Smoke Test

```bash
curl -s http://127.0.0.1:8844/health | jq

curl -s http://127.0.0.1:8844/embed \
  -H 'content-type: application/json' \
  -d '{"texts":["hello","world"],"max_length":64}' | jq '.dimensions,.count'

curl -s http://127.0.0.1:8844/v1/embeddings \
  -H 'content-type: application/json' \
  -d '{"input":["hello","world"]}' | jq '.data | length'
```

## Pipeline Integration

Use the embedding service endpoint from `scoop`:

```bash
cd ~/scoop/backend
go run ./cmd/scoop embed --env .env --endpoint http://127.0.0.1:8844/embed
```

Or for full cycle:

```bash
cd ~/scoop/backend
go run ./cmd/scoop process --env .env --embed-endpoint http://127.0.0.1:8844/embed
```
