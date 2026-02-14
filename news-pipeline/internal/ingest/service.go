package ingest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"horse.fit/news-pipeline/internal/db"
	"horse.fit/news-pipeline/internal/globaltime"
)

const maxIngestErrorLength = 4000

type Service struct {
	pool   *db.Pool
	logger zerolog.Logger
}

type Request struct {
	Source            string
	SourceItemID      string
	Collection        string
	SourceItemURL     string
	SourcePublishedAt *time.Time
	RawPayload        json.RawMessage
	CursorCheckpoint  json.RawMessage
	TriggeredByTopic  string
	ResponseHeaders   json.RawMessage
}

type Result struct {
	RunID          int64
	RunUUID        string
	RawArrivalID   *int64
	RawArrivalUUID *string
	Inserted       bool
	PayloadHashHex string
	Status         string
}

func NewService(pool *db.Pool, logger zerolog.Logger) *Service {
	return &Service{
		pool:   pool,
		logger: logger,
	}
}

func (s *Service) IngestOne(ctx context.Context, req Request) (Result, error) {
	if s == nil || s.pool == nil {
		return Result{}, fmt.Errorf("ingest service is not initialized")
	}

	source := strings.TrimSpace(req.Source)
	if source == "" {
		return Result{}, fmt.Errorf("source is required")
	}

	sourceItemID := strings.TrimSpace(req.SourceItemID)
	if sourceItemID == "" {
		return Result{}, fmt.Errorf("source_item_id is required")
	}
	collection := strings.TrimSpace(strings.ToLower(req.Collection))
	if collection == "" {
		return Result{}, fmt.Errorf("collection is required")
	}

	payloadCanonical, err := canonicalizeJSON(req.RawPayload)
	if err != nil {
		return Result{}, fmt.Errorf("canonicalize raw payload: %w", err)
	}
	payloadHash := sha256.Sum256(payloadCanonical)

	checkpointCanonical, err := s.resolveCheckpoint(req.CursorCheckpoint, sourceItemID)
	if err != nil {
		return Result{}, fmt.Errorf("resolve checkpoint: %w", err)
	}

	runStart := globaltime.UTC()
	runID, runUUID, err := s.insertRun(ctx, source, normalizeNullableString(req.TriggeredByTopic), string(checkpointCanonical), runStart)
	if err != nil {
		return Result{}, fmt.Errorf("insert ingest run: %w", err)
	}

	insertResult, ingestErr := s.insertRawAndCheckpointTx(
		ctx,
		runID,
		source,
		sourceItemID,
		collection,
		normalizeNullableString(req.SourceItemURL),
		normalizeNullableTime(req.SourcePublishedAt),
		string(payloadCanonical),
		payloadHash[:],
		normalizeNullableJSON(req.ResponseHeaders),
		string(checkpointCanonical),
		globaltime.UTC(),
	)
	if ingestErr != nil {
		failedAt := globaltime.UTC()
		markErr := s.markRunFailed(ctx, runID, ingestErr, failedAt)
		if markErr != nil {
			return Result{}, fmt.Errorf("ingest tx failed (%v); failed to mark run failed: %w", ingestErr, markErr)
		}
		return Result{}, ingestErr
	}

	itemsInserted := 0
	if insertResult.inserted {
		itemsInserted = 1
	}

	finishedAt := globaltime.UTC()
	if err := s.markRunCompleted(ctx, runID, itemsInserted, string(checkpointCanonical), finishedAt); err != nil {
		return Result{}, fmt.Errorf("mark ingest run completed: %w", err)
	}

	s.logger.Info().
		Int64("run_id", runID).
		Str("source", source).
		Str("source_item_id", sourceItemID).
		Bool("inserted", insertResult.inserted).
		Msg("ingest completed")

	return Result{
		RunID:          runID,
		RunUUID:        runUUID,
		RawArrivalID:   insertResult.rawArrivalID,
		RawArrivalUUID: insertResult.rawArrivalUUID,
		Inserted:       insertResult.inserted,
		PayloadHashHex: hex.EncodeToString(payloadHash[:]),
		Status:         "completed",
	}, nil
}

type insertTxResult struct {
	rawArrivalID   *int64
	rawArrivalUUID *string
	inserted       bool
}

func (s *Service) insertRun(
	ctx context.Context,
	source string,
	triggeredByTopic *string,
	cursorCheckpoint string,
	runStart time.Time,
) (int64, string, error) {
	const q = `
INSERT INTO news.ingest_runs (
	source,
	triggered_by_topic,
	started_at,
	status,
	items_fetched,
	items_inserted,
	cursor_checkpoint,
	created_at,
	updated_at
)
VALUES ($1, $2, $3, 'running', 0, 0, $4::jsonb, $3, $3)
RETURNING run_id, ingest_run_uuid
`

	var runID int64
	var runUUID string
	err := s.pool.QueryRow(ctx, q, source, triggeredByTopic, runStart, cursorCheckpoint).Scan(&runID, &runUUID)
	if err != nil {
		return 0, "", err
	}
	return runID, runUUID, nil
}

func (s *Service) insertRawAndCheckpointTx(
	ctx context.Context,
	runID int64,
	source string,
	sourceItemID string,
	collection string,
	sourceItemURL *string,
	sourcePublishedAt *time.Time,
	rawPayload string,
	payloadHash []byte,
	responseHeaders *string,
	checkpoint string,
	now time.Time,
) (insertTxResult, error) {
	tx, err := s.pool.BeginTx(ctx, db.TxOptions{})
	if err != nil {
		return insertTxResult{}, fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	const insertRaw = `
INSERT INTO news.raw_arrivals (
	run_id,
	source,
	source_item_id,
	collection,
	source_item_url,
	source_published_at,
	fetched_at,
	raw_payload,
	payload_hash,
	response_headers,
	created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9, $10::jsonb, $7)
ON CONFLICT (source, source_item_id, payload_hash) DO NOTHING
RETURNING raw_arrival_id, raw_arrival_uuid
`

	var rawArrivalID int64
	var rawArrivalUUID string
	inserted := true
	err = tx.QueryRow(
		ctx,
		insertRaw,
		runID,
		source,
		sourceItemID,
		collection,
		sourceItemURL,
		sourcePublishedAt,
		now,
		rawPayload,
		payloadHash,
		responseHeaders,
	).Scan(&rawArrivalID, &rawArrivalUUID)
	if err != nil {
		if db.IsNoRows(err) {
			inserted = false
		} else {
			return insertTxResult{}, fmt.Errorf("insert raw_arrivals: %w", err)
		}
	}

	const upsertCheckpoint = `
INSERT INTO news.source_checkpoints (
	source,
	cursor_checkpoint,
	last_successful_run_id,
	updated_at
)
VALUES ($1, $2::jsonb, $3, $4)
ON CONFLICT (source) DO UPDATE
SET
	cursor_checkpoint = EXCLUDED.cursor_checkpoint,
	last_successful_run_id = EXCLUDED.last_successful_run_id,
	updated_at = EXCLUDED.updated_at
`
	if _, err := tx.Exec(ctx, upsertCheckpoint, source, checkpoint, runID, now); err != nil {
		return insertTxResult{}, fmt.Errorf("upsert source_checkpoints: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return insertTxResult{}, fmt.Errorf("commit transaction: %w", err)
	}

	if !inserted {
		return insertTxResult{
			rawArrivalID:   nil,
			rawArrivalUUID: nil,
			inserted:       false,
		}, nil
	}

	return insertTxResult{
		rawArrivalID:   &rawArrivalID,
		rawArrivalUUID: &rawArrivalUUID,
		inserted:       true,
	}, nil
}

func (s *Service) markRunCompleted(
	ctx context.Context,
	runID int64,
	itemsInserted int,
	checkpoint string,
	finishedAt time.Time,
) error {
	const q = `
UPDATE news.ingest_runs
SET
	status = 'completed',
	items_fetched = 1,
	items_inserted = $2,
	cursor_checkpoint = $3::jsonb,
	finished_at = $4,
	updated_at = $4,
	error_message = NULL
WHERE run_id = $1
`
	_, err := s.pool.Exec(ctx, q, runID, itemsInserted, checkpoint, finishedAt)
	return err
}

func (s *Service) markRunFailed(ctx context.Context, runID int64, cause error, finishedAt time.Time) error {
	const q = `
UPDATE news.ingest_runs
SET
	status = 'failed',
	error_message = $2,
	finished_at = $3,
	updated_at = $3
WHERE run_id = $1
`

	msg := strings.TrimSpace(cause.Error())
	if len(msg) > maxIngestErrorLength {
		msg = msg[:maxIngestErrorLength]
	}

	_, err := s.pool.Exec(ctx, q, runID, msg, finishedAt)
	return err
}

func (s *Service) resolveCheckpoint(checkpoint json.RawMessage, sourceItemID string) ([]byte, error) {
	if len(bytes.TrimSpace(checkpoint)) > 0 {
		return canonicalizeJSON(checkpoint)
	}

	return canonicalizeJSON([]byte(fmt.Sprintf(`{"last_source_item_id":%q}`, sourceItemID)))
}

func canonicalizeJSON(raw []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("JSON payload is empty")
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("JSON contains trailing content")
	}

	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical JSON: %w", err)
	}
	return canonical, nil
}

func normalizeNullableString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeNullableTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	normalized := t.UTC()
	return &normalized
}

func normalizeNullableJSON(value json.RawMessage) *string {
	if len(bytes.TrimSpace(value)) == 0 {
		return nil
	}
	normalized := string(value)
	return &normalized
}
