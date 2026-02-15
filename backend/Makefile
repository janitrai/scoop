# Scoop - Go Worker Makefile

BINARY_NAME=scoop
MAIN_PATH=cmd/scoop/main.go
BUILD_DIR=bin

.DEFAULT_GOAL := help

CYAN := \033[36m
RESET := \033[0m
GREEN := \033[32m
YELLOW := \033[33m

## help: Display this help message
.PHONY: help
help:
	@echo "$(CYAN)Scoop - Available Commands:$(RESET)"
	@echo ""
	@grep -E '^## [a-zA-Z_-]+:' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = "## "}; {printf "  $(GREEN)%-20s$(RESET) %s\n", $$2, $$3}' | sed 's/: / - /'

## deps: Download and verify dependencies
.PHONY: deps
deps:
	@echo "$(CYAN)Installing dependencies...$(RESET)"
	go mod download
	go mod tidy
	go mod verify

## build: Build the CLI binary
.PHONY: build
build:
	@echo "$(CYAN)Building binary...$(RESET)"
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

## run-health: Run DB health check
.PHONY: run-health
run-health:
	@echo "$(CYAN)Running health check...$(RESET)"
	go run $(MAIN_PATH) health

## run-serve: Start Echo API server on 0.0.0.0:8090
.PHONY: run-serve
run-serve:
	@echo "$(CYAN)Starting web server on 0.0.0.0:8090...$(RESET)"
	go run $(MAIN_PATH) serve --host 0.0.0.0 --port 8090

## dev: Run backend with hot reload (requires air)
.PHONY: dev
dev:
	@echo "$(CYAN)Starting backend with hot reload (air)...$(RESET)"
	@which air > /dev/null || (echo "$(YELLOW)Installing air...$(RESET)" && go install github.com/air-verse/air@latest)
	@rm -rf tmp/
	@export PATH="$$(go env GOPATH)/bin:$$PATH" && air -c .air.toml

## web-build: Build React (Vite + TypeScript) frontend static bundle into ../frontend/dist
.PHONY: web-build
web-build:
	@echo "$(CYAN)Building frontend assets...$(RESET)"
	pnpm --dir ../frontend run build

## web-dev: Run Vite frontend dev server
.PHONY: web-dev
web-dev:
	@echo "$(CYAN)Starting frontend dev server...$(RESET)"
	pnpm --dir ../frontend run dev

## run-ingest-smoke: Run one manual ingest event
.PHONY: run-ingest-smoke
run-ingest-smoke:
	@echo "$(CYAN)Running ingest smoke command...$(RESET)"
	go run $(MAIN_PATH) ingest \
		--payload '{"payload_version":"v1","source":"smoke_manual","source_item_id":"smoke-manual-1","title":"Smoke ingest event","source_metadata":{"collection":"smoke_manual","job_name":"make-smoke","job_run_id":"smoke-manual-1","scraped_at":"2026-02-14T00:00:00Z","kind":"smoke"}}'

## run-normalize: Normalize pending raw arrivals into documents
.PHONY: run-normalize
run-normalize:
	@echo "$(CYAN)Running normalize stage...$(RESET)"
	go run $(MAIN_PATH) normalize --limit 1000

## run-embed: Generate embeddings for pending documents
.PHONY: run-embed
run-embed:
	@echo "$(CYAN)Running embed stage...$(RESET)"
	go run $(MAIN_PATH) embed --limit 1000 --batch-size 32

## run-embedding-service: Start local embedding service on :8844 (transformers backend)
.PHONY: run-embedding-service
run-embedding-service:
	@echo "$(CYAN)Starting embedding service (transformers) on 0.0.0.0:8844...$(RESET)"
	cd ../embedding-service && ./.venv/bin/python main.py --server --backend transformers --host 0.0.0.0 --port 8844

## run-embedding-service-mock: Start deterministic embedding service on :8844
.PHONY: run-embedding-service-mock
run-embedding-service-mock:
	@echo "$(CYAN)Starting embedding service (deterministic) on 0.0.0.0:8844...$(RESET)"
	cd ../embedding-service && ./.venv/bin/python main.py --server --backend deterministic --host 0.0.0.0 --port 8844

## run-validate-testdata: Validate testdata JSON files against v1 schema
.PHONY: run-validate-testdata
run-validate-testdata:
	@echo "$(CYAN)Validating testdata/news_items...$(RESET)"
	go run $(MAIN_PATH) validate --dir testdata/news_items --recursive=true

## run-dedup: Deduplicate pending documents into stories
.PHONY: run-dedup
run-dedup:
	@echo "$(CYAN)Running dedup stage...$(RESET)"
	go run $(MAIN_PATH) dedup --limit 1000

## run-process: Run normalize + dedup until empty
.PHONY: run-process
run-process:
	@echo "$(CYAN)Running process stage (normalize + embed + dedup)...$(RESET)"
	go run $(MAIN_PATH) process --normalize-limit 1000 --embed-limit 1000 --embed-batch-size 32 --dedup-limit 1000 --until-empty=true --max-cycles 25

## bench-dedup-perf: Run repeatable dedup performance benchmark on testdata/scraped_news
.PHONY: bench-dedup-perf
bench-dedup-perf:
	@echo "$(CYAN)Running dedup performance benchmark...$(RESET)"
	bash scripts/bench_dedup_performance.sh

## eval-dedup-gt: Evaluate dedup output against manual ground truth annotations
.PHONY: eval-dedup-gt
eval-dedup-gt:
	@echo "$(CYAN)Evaluating dedup against ground truth...$(RESET)"
	bash scripts/eval_dedup_ground_truth.sh

## fmt: Format Go code
.PHONY: fmt
fmt:
	@echo "$(CYAN)Formatting code...$(RESET)"
	go fmt ./...

## vet: Run go vet
.PHONY: vet
vet:
	@echo "$(CYAN)Running go vet...$(RESET)"
	go vet ./...

## test: Run tests
.PHONY: test
test:
	@echo "$(CYAN)Running tests...$(RESET)"
	go test ./...

## lint: Run golangci-lint (requires golangci-lint)
.PHONY: lint
lint:
	@echo "$(CYAN)Running linter...$(RESET)"
	@which golangci-lint > /dev/null || (echo "$(YELLOW)Install golangci-lint first$(RESET)" && exit 1)
	golangci-lint run

## clean: Remove build artifacts
.PHONY: clean
clean:
	@echo "$(CYAN)Cleaning artifacts...$(RESET)"
	rm -rf $(BUILD_DIR) tmp coverage.out coverage.html
