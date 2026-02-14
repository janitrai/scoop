package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"horse.fit/scoop/internal/db"
	"horse.fit/scoop/internal/globaltime"
)

const (
	DefaultEmbeddingEndpoint       = "http://127.0.0.1:8844/embed"
	DefaultEmbeddingModelName      = "Qwen3-Embedding-8B"
	DefaultEmbeddingModelVersion   = "v1"
	DefaultEmbeddingBatchSize      = 32
	DefaultEmbeddingMaxLength      = 512
	DefaultEmbeddingRequestTimeout = 45 * time.Second
	embeddingVectorDimensions      = 4096
)

type EmbedOptions struct {
	Limit          int
	BatchSize      int
	Endpoint       string
	ModelName      string
	ModelVersion   string
	MaxLength      int
	RequestTimeout time.Duration
}

type EmbedResult struct {
	Processed int
	Embedded  int
	Skipped   int
	Failed    int
}

type embeddingPendingDocument struct {
	DocumentID      int64
	NormalizedTitle string
	NormalizedText  string
}

type embedRequest struct {
	Texts     []string `json:"texts,omitempty"`
	Input     []string `json:"input,omitempty"`
	MaxLength int      `json:"max_length,omitempty"`
}

type embedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
	ElapsedMS  *float64    `json:"elapsed_ms"`
	Data       []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func (s *Service) EmbedPending(ctx context.Context, options EmbedOptions) (EmbedResult, error) {
	if s == nil || s.pool == nil {
		return EmbedResult{}, fmt.Errorf("pipeline service is not initialized")
	}

	opts := normalizeEmbedOptions(options)
	if opts.Limit <= 0 {
		return EmbedResult{}, nil
	}

	var result EmbedResult
	for result.Processed < opts.Limit {
		remaining := opts.Limit - result.Processed
		batchSize := min(opts.BatchSize, remaining)

		documents, err := selectPendingEmbeddingDocuments(ctx, s.pool, opts.ModelName, opts.ModelVersion, batchSize)
		if err != nil {
			return result, err
		}
		if len(documents) == 0 {
			break
		}

		texts := make([]string, 0, len(documents))
		for _, document := range documents {
			texts = append(texts, embeddingInput(document))
		}

		vectors, _, err := requestEmbeddings(ctx, opts, texts)
		if err != nil {
			return result, err
		}
		if len(vectors) != len(documents) {
			return result, fmt.Errorf("embedding response count mismatch: requested=%d returned=%d", len(documents), len(vectors))
		}

		for i, document := range documents {
			result.Processed++

			vectorLiteral, err := toVectorLiteral(vectors[i])
			if err != nil {
				result.Failed++
				return result, fmt.Errorf("document_id=%d invalid embedding vector: %w", document.DocumentID, err)
			}

			inserted, err := insertDocumentEmbedding(
				ctx,
				s.pool,
				document.DocumentID,
				opts.ModelName,
				opts.ModelVersion,
				vectorLiteral,
				opts.Endpoint,
				globaltime.UTC(),
			)
			if err != nil {
				result.Failed++
				return result, err
			}

			if inserted {
				result.Embedded++
			} else {
				result.Skipped++
			}
		}
	}

	return result, nil
}

func normalizeEmbedOptions(opts EmbedOptions) EmbedOptions {
	normalized := opts
	if normalized.Limit < 0 {
		normalized.Limit = 0
	}
	if normalized.BatchSize <= 0 {
		normalized.BatchSize = DefaultEmbeddingBatchSize
	}
	if normalized.BatchSize > normalized.Limit && normalized.Limit > 0 {
		normalized.BatchSize = normalized.Limit
	}
	if strings.TrimSpace(normalized.Endpoint) == "" {
		normalized.Endpoint = DefaultEmbeddingEndpoint
	}
	normalized.Endpoint = normalizeEmbeddingEndpoint(normalized.Endpoint)
	if strings.TrimSpace(normalized.ModelName) == "" {
		normalized.ModelName = DefaultEmbeddingModelName
	}
	if strings.TrimSpace(normalized.ModelVersion) == "" {
		normalized.ModelVersion = DefaultEmbeddingModelVersion
	}
	if normalized.MaxLength <= 0 {
		normalized.MaxLength = DefaultEmbeddingMaxLength
	}
	if normalized.RequestTimeout <= 0 {
		normalized.RequestTimeout = DefaultEmbeddingRequestTimeout
	}
	return normalized
}

func selectPendingEmbeddingDocuments(
	ctx context.Context,
	pool *db.Pool,
	modelName string,
	modelVersion string,
	limit int,
) ([]embeddingPendingDocument, error) {
	const q = `
SELECT
	d.document_id,
	d.normalized_title,
	d.normalized_text
FROM news.documents d
WHERE NOT EXISTS (
	SELECT 1
	FROM news.document_embeddings de
	WHERE de.document_id = d.document_id
	  AND de.model_name = $1
	  AND de.model_version = $2
)
ORDER BY d.document_id
LIMIT $3
`

	rows, err := pool.Query(ctx, q, modelName, modelVersion, limit)
	if err != nil {
		return nil, fmt.Errorf("select pending documents for embedding: %w", err)
	}
	defer rows.Close()

	documents := make([]embeddingPendingDocument, 0, limit)
	for rows.Next() {
		var document embeddingPendingDocument
		if err := rows.Scan(&document.DocumentID, &document.NormalizedTitle, &document.NormalizedText); err != nil {
			return nil, fmt.Errorf("scan pending embedding document: %w", err)
		}
		documents = append(documents, document)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending embedding documents: %w", err)
	}
	return documents, nil
}

func insertDocumentEmbedding(
	ctx context.Context,
	pool *db.Pool,
	documentID int64,
	modelName string,
	modelVersion string,
	vectorLiteral string,
	endpoint string,
	now time.Time,
) (bool, error) {
	const q = `
INSERT INTO news.document_embeddings (
	document_id,
	model_name,
	model_version,
	embedding,
	embedded_at,
	service_endpoint
)
VALUES ($1, $2, $3, $4::vector, $5, $6)
ON CONFLICT (document_id, model_name, model_version) DO NOTHING
`

	tag, err := pool.Exec(ctx, q, documentID, modelName, modelVersion, vectorLiteral, now, endpoint)
	if err != nil {
		return false, fmt.Errorf("insert document embedding document_id=%d: %w", documentID, err)
	}
	return tag.RowsAffected() == 1, nil
}

func embeddingInput(document embeddingPendingDocument) string {
	title := strings.TrimSpace(document.NormalizedTitle)
	body := strings.TrimSpace(document.NormalizedText)
	switch {
	case title == "" && body == "":
		return ""
	case body == "":
		return title
	case title == "":
		return body
	default:
		return title + "\n\n" + body
	}
}

func requestEmbeddings(ctx context.Context, opts EmbedOptions, texts []string) ([][]float64, *float64, error) {
	payload := embedRequest{
		Texts:     texts,
		MaxLength: opts.MaxLength,
	}

	parsedEndpoint, err := url.Parse(opts.Endpoint)
	if err == nil && strings.HasSuffix(parsedEndpoint.Path, "/v1/embeddings") {
		payload = embedRequest{
			Input: texts,
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	requestCtx, cancel := context.WithTimeout(ctx, opts.RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, opts.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("build embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read embedding response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("embedding service status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed embedResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, nil, fmt.Errorf("decode embedding response: %w", err)
	}

	vectors := parsed.Embeddings
	if len(vectors) == 0 && len(parsed.Data) > 0 {
		sort.Slice(parsed.Data, func(i, j int) bool {
			return parsed.Data[i].Index < parsed.Data[j].Index
		})
		vectors = make([][]float64, 0, len(parsed.Data))
		for _, row := range parsed.Data {
			vectors = append(vectors, row.Embedding)
		}
	}
	if len(vectors) == 0 {
		return nil, parsed.ElapsedMS, fmt.Errorf("embedding response missing vectors")
	}

	return vectors, parsed.ElapsedMS, nil
}

func normalizeEmbeddingEndpoint(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return DefaultEmbeddingEndpoint
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/embed"
	}
	return parsed.String()
}

func toVectorLiteral(values []float64) (string, error) {
	if len(values) != embeddingVectorDimensions {
		return "", fmt.Errorf("expected %d dimensions, got %d", embeddingVectorDimensions, len(values))
	}

	var builder strings.Builder
	builder.Grow(len(values) * 8)
	builder.WriteByte('[')
	for i, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return "", fmt.Errorf("vector has non-finite value at index %d", i)
		}
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(strconv.FormatFloat(value, 'f', -1, 64))
	}
	builder.WriteByte(']')
	return builder.String(), nil
}
