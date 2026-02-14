package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"math/bits"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/rs/zerolog"

	"horse.fit/scoop/internal/db"
	"horse.fit/scoop/internal/globaltime"
	payloadschema "horse.fit/scoop/schema"
)

const (
	DefaultDedupLookbackDays         = 365
	defaultSemanticCandidateLimit    = 20
	defaultSemanticSearchEF          = 64
	defaultLexicalSimhashMaxDistance = 3
	defaultLexicalTrigramThreshold   = 0.88
	defaultLexicalTrigramDateWindow  = 14 * 24 * time.Hour
	defaultSemanticAutoMergeCosine   = 0.935
	defaultSemanticOverrideCosine    = 0.965
	defaultSemanticTitleOverlapFloor = 0.30
	defaultSemanticGrayZoneMinCosine = 0.89
	semanticCompositeCosineWeight    = 0.75
	semanticCompositeTitleWeight     = 0.15
	semanticCompositeDateWeight      = 0.10
	storyCandidateLimit              = 300
)

var trackingQueryKeys = map[string]struct{}{
	"fbclid":  {},
	"gclid":   {},
	"mc_cid":  {},
	"mc_eid":  {},
	"ref":     {},
	"ref_src": {},
}

type Service struct {
	pool   *db.Pool
	logger zerolog.Logger
}

type NormalizeResult struct {
	Processed int
	Inserted  int
}

type DedupResult struct {
	Processed  int
	NewStories int
	AutoMerges int
	GrayZones  int
}

type DedupOptions struct {
	Limit        int
	ModelName    string
	ModelVersion string
	LookbackDays int
}

type rawArrivalRow struct {
	RawArrivalID      int64
	Source            string
	SourceItemID      string
	Collection        string
	SourceItemURL     *string
	SourcePublishedAt *time.Time
	RawPayload        []byte
	FetchedAt         time.Time
}

type normalizedDocument struct {
	RawArrivalID      int64
	Source            string
	SourceItemID      string
	Collection        string
	CanonicalURL      *string
	CanonicalURLHash  []byte
	NormalizedTitle   string
	NormalizedText    string
	NormalizedLang    string
	PublishedAt       *time.Time
	SourceDomain      *string
	TitleSimhash      *int64
	TextSimhash       *int64
	TitleHash         []byte
	ContentHash       []byte
	TokenCount        int
	DocumentCreatedAt time.Time
}

type pendingDocument struct {
	DocumentID        int64
	Source            string
	SourceItemID      string
	Collection        string
	CanonicalURL      *string
	CanonicalURLHash  []byte
	NormalizedTitle   string
	NormalizedText    string
	PublishedAt       *time.Time
	SourceDomain      *string
	TitleSimhash      *int64
	ContentHash       []byte
	EmbeddingVector   *string
	DocumentCreatedAt time.Time
}

type storyCandidate struct {
	StoryID      int64
	Title        string
	LastSeenAt   time.Time
	SourceCount  int
	ItemCount    int
	CanonicalURL *string
	TitleSimhash *int64
}

type semanticCandidate struct {
	StoryID    int64
	Title      string
	LastSeenAt time.Time
	Cosine     float64
}

type lexicalAutoMergeMatch struct {
	Candidate       storyCandidate
	Signal          string
	MatchScore      float64
	TitleOverlap    float64
	DateConsistency float64
	CompositeScore  float64
	SimhashDistance *int
}

type semanticMatch struct {
	Candidate       semanticCandidate
	TitleOverlap    float64
	DateConsistency float64
	CompositeScore  float64
}

type dedupDecisionKind string

const (
	decisionNone      dedupDecisionKind = ""
	decisionNewStory  dedupDecisionKind = "new_story"
	decisionAutoMerge dedupDecisionKind = "auto_merge"
	decisionGrayZone  dedupDecisionKind = "gray_zone"
)

func NewService(pool *db.Pool, logger zerolog.Logger) *Service {
	return &Service{
		pool:   pool,
		logger: logger,
	}
}

func (s *Service) NormalizePending(ctx context.Context, limit int) (NormalizeResult, error) {
	if s == nil || s.pool == nil {
		return NormalizeResult{}, fmt.Errorf("pipeline service is not initialized")
	}
	if limit <= 0 {
		return NormalizeResult{}, nil
	}

	var result NormalizeResult
	for result.Processed < limit {
		tx, err := s.pool.BeginTx(ctx, db.TxOptions{})
		if err != nil {
			return result, fmt.Errorf("begin normalize tx: %w", err)
		}

		row, found, err := claimOnePendingRawArrivalTx(ctx, tx)
		if err != nil {
			_ = tx.Rollback(ctx)
			return result, err
		}
		if !found {
			if err := tx.Commit(ctx); err != nil {
				_ = tx.Rollback(ctx)
				return result, fmt.Errorf("commit empty normalize tx: %w", err)
			}
			break
		}

		doc := buildNormalizedDocument(row, s.logger)
		inserted, err := insertDocumentTx(ctx, tx, doc)
		if err != nil {
			_ = tx.Rollback(ctx)
			return result, err
		}

		if err := tx.Commit(ctx); err != nil {
			_ = tx.Rollback(ctx)
			return result, fmt.Errorf("commit normalize tx: %w", err)
		}

		result.Processed++
		if inserted {
			result.Inserted++
		}
	}

	return result, nil
}

func (s *Service) DedupPending(ctx context.Context, opts DedupOptions) (DedupResult, error) {
	if s == nil || s.pool == nil {
		return DedupResult{}, fmt.Errorf("pipeline service is not initialized")
	}
	if opts.Limit <= 0 {
		return DedupResult{}, nil
	}

	modelName := strings.TrimSpace(opts.ModelName)
	if modelName == "" {
		modelName = DefaultEmbeddingModelName
	}
	modelVersion := strings.TrimSpace(opts.ModelVersion)
	if modelVersion == "" {
		modelVersion = DefaultEmbeddingModelVersion
	}
	lookbackDays := opts.LookbackDays
	if lookbackDays <= 0 {
		lookbackDays = DefaultDedupLookbackDays
	}
	lookbackCutoff := globaltime.UTC().AddDate(0, 0, -lookbackDays)

	var result DedupResult
	for result.Processed < opts.Limit {
		tx, err := s.pool.BeginTx(ctx, db.TxOptions{})
		if err != nil {
			return result, fmt.Errorf("begin dedup tx: %w", err)
		}

		doc, found, err := claimOnePendingDocumentTx(ctx, tx, modelName, modelVersion)
		if err != nil {
			_ = tx.Rollback(ctx)
			return result, err
		}
		if !found {
			if err := tx.Commit(ctx); err != nil {
				_ = tx.Rollback(ctx)
				return result, fmt.Errorf("commit empty dedup tx: %w", err)
			}
			break
		}

		decision, err := dedupDocumentTx(ctx, tx, doc, modelName, modelVersion, lookbackCutoff)
		if err != nil {
			_ = tx.Rollback(ctx)
			return result, err
		}

		if err := tx.Commit(ctx); err != nil {
			_ = tx.Rollback(ctx)
			return result, fmt.Errorf("commit dedup tx: %w", err)
		}

		if decision == decisionNone {
			continue
		}

		result.Processed++
		switch decision {
		case decisionNewStory:
			result.NewStories++
		case decisionAutoMerge:
			result.AutoMerges++
		case decisionGrayZone:
			result.GrayZones++
		}
	}

	return result, nil
}

func claimOnePendingRawArrivalTx(ctx context.Context, tx db.Tx) (rawArrivalRow, bool, error) {
	const q = `
SELECT
	ra.raw_arrival_id,
	ra.source,
	ra.source_item_id,
	ra.collection,
	ra.source_item_url,
	ra.source_published_at,
	ra.raw_payload,
	ra.fetched_at
FROM news.raw_arrivals ra
WHERE NOT EXISTS (
	SELECT 1
	FROM news.documents d
	WHERE d.raw_arrival_id = ra.raw_arrival_id
)
ORDER BY ra.raw_arrival_id
LIMIT 1
FOR UPDATE SKIP LOCKED
`

	var row rawArrivalRow
	var sourceItemURL *string
	var sourcePublishedAt *time.Time
	err := tx.QueryRow(ctx, q).Scan(
		&row.RawArrivalID,
		&row.Source,
		&row.SourceItemID,
		&row.Collection,
		&sourceItemURL,
		&sourcePublishedAt,
		&row.RawPayload,
		&row.FetchedAt,
	)
	if err != nil {
		if err == db.ErrNoRows {
			return rawArrivalRow{}, false, nil
		}
		return rawArrivalRow{}, false, fmt.Errorf("claim raw_arrival: %w", err)
	}

	row.SourceItemURL = sourceItemURL
	row.SourcePublishedAt = sourcePublishedAt
	return row, true, nil
}

func insertDocumentTx(ctx context.Context, tx db.Tx, doc normalizedDocument) (bool, error) {
	const q = `
INSERT INTO news.documents (
	raw_arrival_id,
	source,
	source_item_id,
	collection,
	canonical_url,
	canonical_url_hash,
	normalized_title,
	normalized_text,
	normalized_language,
	published_at,
	source_domain,
	title_simhash,
	text_simhash,
	title_hash,
	content_hash,
	token_count,
	created_at,
	updated_at
)
VALUES (
	$1,
	$2,
	$3,
	$4,
	$5,
	$6,
	$7,
	$8,
	$9,
	$10,
	$11,
	$12,
	$13,
	$14,
	$15,
	$16,
	$17,
	$17
)
ON CONFLICT (raw_arrival_id) DO NOTHING
`

	commandTag, err := tx.Exec(
		ctx,
		q,
		doc.RawArrivalID,
		doc.Source,
		doc.SourceItemID,
		doc.Collection,
		doc.CanonicalURL,
		nullableBytes(doc.CanonicalURLHash),
		doc.NormalizedTitle,
		doc.NormalizedText,
		doc.NormalizedLang,
		doc.PublishedAt,
		doc.SourceDomain,
		doc.TitleSimhash,
		doc.TextSimhash,
		doc.TitleHash,
		doc.ContentHash,
		doc.TokenCount,
		doc.DocumentCreatedAt,
	)
	if err != nil {
		return false, fmt.Errorf("insert document raw_arrival_id=%d: %w", doc.RawArrivalID, err)
	}
	return commandTag.RowsAffected() == 1, nil
}

func buildNormalizedDocument(row rawArrivalRow, logger zerolog.Logger) normalizedDocument {
	now := globaltime.UTC()
	if row.FetchedAt.IsZero() {
		row.FetchedAt = now
	}

	source := strings.TrimSpace(row.Source)
	sourceItemID := strings.TrimSpace(row.SourceItemID)

	var (
		title        string
		bodyText     string
		language     string
		canonicalURL string
		sourceDomain string
		collection   string
		publishedAt  *time.Time
	)

	item, err := payloadschema.ValidateNewsItemPayload(row.RawPayload)
	if err != nil {
		logger.Warn().
			Err(err).
			Int64("raw_arrival_id", row.RawArrivalID).
			Msg("payload schema validation failed during normalize; falling back to lenient extraction")
	} else {
		title = strings.TrimSpace(item.Title)
		if item.BodyText != nil {
			bodyText = strings.TrimSpace(*item.BodyText)
		}
		if item.Language != nil {
			language = strings.TrimSpace(strings.ToLower(*item.Language))
		}
		if item.CanonicalURL != nil {
			canonicalURL = strings.TrimSpace(*item.CanonicalURL)
		}
		if item.SourceDomain != nil {
			sourceDomain = strings.TrimSpace(strings.ToLower(*item.SourceDomain))
		}
		collection = extractCollectionFromMetadata(item.SourceMetadata)
		if item.PublishedAt != nil {
			if ts, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(*item.PublishedAt)); parseErr == nil {
				utc := ts.UTC()
				publishedAt = &utc
			}
		}
	}

	if title == "" {
		title = sourceItemID
	}
	if language == "" {
		language = "und"
	}
	if publishedAt == nil && row.SourcePublishedAt != nil {
		utc := row.SourcePublishedAt.UTC()
		publishedAt = &utc
	}
	if canonicalURL == "" && row.SourceItemURL != nil {
		canonicalURL = strings.TrimSpace(*row.SourceItemURL)
	}
	if collection == "" {
		collection = normalizeCollectionLabel(row.Collection)
	}
	if collection == "" {
		collection = "unknown"
	}

	normalizedCanonicalURL, host := normalizeURL(canonicalURL)
	if sourceDomain == "" {
		sourceDomain = host
	}

	normalizedTitle := normalizeText(title)
	if normalizedTitle == "" {
		normalizedTitle = sourceItemID
	}
	normalizedBody := normalizeText(bodyText)

	titleHash := sha256.Sum256([]byte(normalizedTitle))
	contentHash := sha256.Sum256([]byte(normalizedTitle + "\n" + normalizedBody))

	var canonicalURLHash []byte
	var canonicalURLPtr *string
	if normalizedCanonicalURL != "" {
		hash := sha256.Sum256([]byte(normalizedCanonicalURL))
		canonicalURLHash = append([]byte(nil), hash[:]...)
		canonicalURLCopy := normalizedCanonicalURL
		canonicalURLPtr = &canonicalURLCopy
	}

	var sourceDomainPtr *string
	if sourceDomain != "" {
		sourceDomainCopy := sourceDomain
		sourceDomainPtr = &sourceDomainCopy
	}

	var titleSimhashPtr *int64
	if v, ok := simhash64(normalizedTitle); ok {
		value := int64(v)
		titleSimhashPtr = &value
	}

	var textSimhashPtr *int64
	if v, ok := simhash64(normalizedBody); ok {
		value := int64(v)
		textSimhashPtr = &value
	}

	return normalizedDocument{
		RawArrivalID:      row.RawArrivalID,
		Source:            source,
		SourceItemID:      sourceItemID,
		Collection:        collection,
		CanonicalURL:      canonicalURLPtr,
		CanonicalURLHash:  canonicalURLHash,
		NormalizedTitle:   normalizedTitle,
		NormalizedText:    normalizedBody,
		NormalizedLang:    language,
		PublishedAt:       publishedAt,
		SourceDomain:      sourceDomainPtr,
		TitleSimhash:      titleSimhashPtr,
		TextSimhash:       textSimhashPtr,
		TitleHash:         append([]byte(nil), titleHash[:]...),
		ContentHash:       append([]byte(nil), contentHash[:]...),
		TokenCount:        countTokens(normalizedTitle + " " + normalizedBody),
		DocumentCreatedAt: row.FetchedAt.UTC(),
	}
}

func claimOnePendingDocumentTx(ctx context.Context, tx db.Tx, modelName, modelVersion string) (pendingDocument, bool, error) {
	const q = `
SELECT
	d.document_id,
	d.source,
	d.source_item_id,
	d.collection,
	d.canonical_url,
	d.canonical_url_hash,
	d.normalized_title,
	d.normalized_text,
	d.published_at,
	d.source_domain,
	d.title_simhash,
	d.content_hash,
	de.embedding::text,
	d.created_at
FROM news.documents d
JOIN news.document_embeddings de
	ON de.document_id = d.document_id
	AND de.model_name = $1
	AND de.model_version = $2
WHERE NOT EXISTS (
	SELECT 1
	FROM news.story_members sm
	WHERE sm.document_id = d.document_id
)
ORDER BY d.document_id
LIMIT 1
FOR UPDATE OF d SKIP LOCKED
`

	var row pendingDocument
	var canonicalURL *string
	var publishedAt *time.Time
	var sourceDomain *string
	var titleSimhash *int64
	var embeddingVector string
	err := tx.QueryRow(ctx, q, modelName, modelVersion).Scan(
		&row.DocumentID,
		&row.Source,
		&row.SourceItemID,
		&row.Collection,
		&canonicalURL,
		&row.CanonicalURLHash,
		&row.NormalizedTitle,
		&row.NormalizedText,
		&publishedAt,
		&sourceDomain,
		&titleSimhash,
		&row.ContentHash,
		&embeddingVector,
		&row.DocumentCreatedAt,
	)
	if err != nil {
		if err == db.ErrNoRows {
			return pendingDocument{}, false, nil
		}
		return pendingDocument{}, false, fmt.Errorf("claim pending document: %w", err)
	}

	row.CanonicalURL = canonicalURL
	row.PublishedAt = publishedAt
	row.SourceDomain = sourceDomain
	row.TitleSimhash = titleSimhash
	if strings.TrimSpace(embeddingVector) != "" {
		embeddingCopy := embeddingVector
		row.EmbeddingVector = &embeddingCopy
	}
	return row, true, nil
}

func dedupDocumentTx(
	ctx context.Context,
	tx db.Tx,
	doc pendingDocument,
	modelName string,
	modelVersion string,
	lookbackCutoff time.Time,
) (dedupDecisionKind, error) {
	now := globaltime.UTC()
	documentSeenAt := doc.DocumentCreatedAt
	if doc.PublishedAt != nil && !doc.PublishedAt.IsZero() {
		documentSeenAt = doc.PublishedAt.UTC()
	}

	if storyID, found, err := findExactURLStoryTx(ctx, tx, doc.Collection, doc.CanonicalURLHash); err != nil {
		return decisionNone, err
	} else if found {
		return applyAutoMergeTx(ctx, tx, doc, storyID, "exact_url", 1, map[string]any{
			"signal": "exact_url",
		}, now, documentSeenAt)
	}

	if storyID, found, err := findExactSourceIDStoryTx(ctx, tx, doc.Collection, doc.Source, doc.SourceItemID); err != nil {
		return decisionNone, err
	} else if found {
		return applyAutoMergeTx(ctx, tx, doc, storyID, "exact_source_id", 1, map[string]any{
			"signal": "exact_source_id",
		}, now, documentSeenAt)
	}

	if storyID, found, err := findExactContentHashStoryTx(ctx, tx, doc.Collection, doc.ContentHash); err != nil {
		return decisionNone, err
	} else if found {
		return applyAutoMergeTx(ctx, tx, doc, storyID, "exact_content_hash", 1, map[string]any{
			"signal": "exact_content_hash",
		}, now, documentSeenAt)
	}

	lexicalMatch, hasLexicalAutoMerge, err := findLexicalAutoMergeTx(ctx, tx, doc, lookbackCutoff)
	if err != nil {
		return decisionNone, err
	}
	if hasLexicalAutoMerge {
		matchDetails := map[string]any{
			"signal":           lexicalMatch.Signal,
			"title_overlap":    lexicalMatch.TitleOverlap,
			"date_consistency": lexicalMatch.DateConsistency,
			"composite_score":  lexicalMatch.CompositeScore,
			"match_score":      lexicalMatch.MatchScore,
		}
		if lexicalMatch.SimhashDistance != nil {
			matchDetails["simhash_distance"] = *lexicalMatch.SimhashDistance
		}
		return applyAutoMergeTx(
			ctx,
			tx,
			doc,
			lexicalMatch.Candidate.StoryID,
			lexicalMatch.Signal,
			lexicalMatch.MatchScore,
			matchDetails,
			now,
			documentSeenAt,
		)
	}

	var bestSemantic *semanticMatch
	if doc.EmbeddingVector != nil && strings.TrimSpace(*doc.EmbeddingVector) != "" {
		candidates, err := findSemanticCandidatesTx(
			ctx,
			tx,
			strings.TrimSpace(*doc.EmbeddingVector),
			doc.Collection,
			modelName,
			modelVersion,
			lookbackCutoff,
			defaultSemanticCandidateLimit,
		)
		if err != nil {
			return decisionNone, err
		}

		for _, candidate := range candidates {
			titleOverlap := titleTokenJaccard(doc.NormalizedTitle, candidate.Title)
			dateConsistency := computeDateConsistency(doc.PublishedAt, candidate.LastSeenAt)
			composite := semanticCompositeScore(candidate.Cosine, titleOverlap, dateConsistency)
			current := semanticMatch{
				Candidate:       candidate,
				TitleOverlap:    titleOverlap,
				DateConsistency: dateConsistency,
				CompositeScore:  composite,
			}
			if bestSemantic == nil || current.CompositeScore > bestSemantic.CompositeScore {
				match := current
				bestSemantic = &match
			}

			if shouldAutoMergeSemantic(candidate.Cosine, titleOverlap) {
				return applySemanticAutoMergeTx(
					ctx,
					tx,
					doc,
					candidate.StoryID,
					candidate.Cosine,
					titleOverlap,
					dateConsistency,
					composite,
					now,
					documentSeenAt,
				)
			}
		}
	}

	newStoryID, err := createStoryTx(ctx, tx, doc, documentSeenAt, now)
	if err != nil {
		return decisionNone, err
	}

	if inserted, err := upsertStoryMemberTx(ctx, tx, newStoryID, doc.DocumentID, "seed", nil, map[string]any{
		"signal": "seed",
	}, now); err != nil {
		return decisionNone, err
	} else if !inserted {
		return decisionNone, nil
	}

	decision := decisionNewStory
	var bestCandidateStoryID *int64
	var titleOverlapPtr *float64
	var entityDateConsistencyPtr *float64
	var compositeScorePtr *float64
	var bestCosinePtr *float64

	if bestSemantic != nil && shouldMarkSemanticGrayZone(bestSemantic.Candidate.Cosine) {
		decision = decisionGrayZone
		storyID := bestSemantic.Candidate.StoryID
		bestCandidateStoryID = &storyID
		bestCosinePtr = floatPtr(bestSemantic.Candidate.Cosine)
		titleOverlapPtr = floatPtr(bestSemantic.TitleOverlap)
		entityDateConsistencyPtr = floatPtr(bestSemantic.DateConsistency)
		compositeScorePtr = floatPtr(bestSemantic.CompositeScore)
	}

	if err := insertDedupEventTx(ctx, tx, dedupEventRecord{
		DocumentID:            doc.DocumentID,
		Decision:              string(decision),
		ChosenStoryID:         &newStoryID,
		BestCandidateStoryID:  bestCandidateStoryID,
		BestCosine:            bestCosinePtr,
		TitleOverlap:          titleOverlapPtr,
		EntityDateConsistency: entityDateConsistencyPtr,
		CompositeScore:        compositeScorePtr,
		ExactSignal:           nil,
		CreatedAt:             now,
	}); err != nil {
		return decisionNone, err
	}

	return decision, nil
}

func applyAutoMergeTx(
	ctx context.Context,
	tx db.Tx,
	doc pendingDocument,
	storyID int64,
	exactSignal string,
	matchScore float64,
	matchDetails map[string]any,
	now time.Time,
	documentSeenAt time.Time,
) (dedupDecisionKind, error) {
	if inserted, err := upsertStoryMemberTx(ctx, tx, storyID, doc.DocumentID, matchTypeForSignal(exactSignal), floatPtr(matchScore), matchDetails, now); err != nil {
		return decisionNone, err
	} else if !inserted {
		return decisionNone, nil
	}

	if err := refreshStoryAggregateTx(ctx, tx, storyID, doc.DocumentID, doc.NormalizedTitle, doc.CanonicalURL, doc.CanonicalURLHash, documentSeenAt, now); err != nil {
		return decisionNone, err
	}

	exactSignalCopy := exactSignal
	if err := insertDedupEventTx(ctx, tx, dedupEventRecord{
		DocumentID:            doc.DocumentID,
		Decision:              string(decisionAutoMerge),
		ChosenStoryID:         &storyID,
		BestCandidateStoryID:  &storyID,
		BestCosine:            nil,
		TitleOverlap:          nil,
		EntityDateConsistency: nil,
		CompositeScore:        floatPtr(matchScore),
		ExactSignal:           &exactSignalCopy,
		CreatedAt:             now,
	}); err != nil {
		return decisionNone, err
	}

	return decisionAutoMerge, nil
}

func applySemanticAutoMergeTx(
	ctx context.Context,
	tx db.Tx,
	doc pendingDocument,
	storyID int64,
	cosine float64,
	titleOverlap float64,
	dateConsistency float64,
	composite float64,
	now time.Time,
	documentSeenAt time.Time,
) (dedupDecisionKind, error) {
	matchDetails := map[string]any{
		"signal":           "semantic",
		"cosine":           cosine,
		"title_overlap":    titleOverlap,
		"date_consistency": dateConsistency,
		"composite_score":  composite,
	}

	if inserted, err := upsertStoryMemberTx(
		ctx,
		tx,
		storyID,
		doc.DocumentID,
		"semantic",
		floatPtr(composite),
		matchDetails,
		now,
	); err != nil {
		return decisionNone, err
	} else if !inserted {
		return decisionNone, nil
	}

	if err := refreshStoryAggregateTx(
		ctx,
		tx,
		storyID,
		doc.DocumentID,
		doc.NormalizedTitle,
		doc.CanonicalURL,
		doc.CanonicalURLHash,
		documentSeenAt,
		now,
	); err != nil {
		return decisionNone, err
	}

	signal := "semantic"
	if err := insertDedupEventTx(ctx, tx, dedupEventRecord{
		DocumentID:            doc.DocumentID,
		Decision:              string(decisionAutoMerge),
		ChosenStoryID:         &storyID,
		BestCandidateStoryID:  &storyID,
		BestCosine:            floatPtr(cosine),
		TitleOverlap:          floatPtr(titleOverlap),
		EntityDateConsistency: floatPtr(dateConsistency),
		CompositeScore:        floatPtr(composite),
		ExactSignal:           &signal,
		CreatedAt:             now,
	}); err != nil {
		return decisionNone, err
	}

	return decisionAutoMerge, nil
}

func matchTypeForSignal(signal string) string {
	switch signal {
	case "exact_url":
		return "exact_url"
	case "exact_source_id":
		return "exact_source_id"
	case "exact_content_hash":
		return "exact_content_hash"
	case "lexical_simhash":
		return "lexical_simhash"
	case "lexical_overlap":
		return "lexical_overlap"
	case "semantic":
		return "semantic"
	default:
		return "manual"
	}
}

func findExactURLStoryTx(ctx context.Context, tx db.Tx, collection string, canonicalURLHash []byte) (int64, bool, error) {
	if len(canonicalURLHash) == 0 {
		return 0, false, nil
	}
	const q = `
SELECT story_id
FROM news.stories
WHERE status = 'active'
  AND collection = $1
  AND canonical_url_hash = $2
ORDER BY last_seen_at DESC
LIMIT 1
`
	var storyID int64
	err := tx.QueryRow(ctx, q, collection, canonicalURLHash).Scan(&storyID)
	if err != nil {
		if err == db.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("find exact_url story: %w", err)
	}
	return storyID, true, nil
}

func findExactSourceIDStoryTx(ctx context.Context, tx db.Tx, collection, source, sourceItemID string) (int64, bool, error) {
	const q = `
SELECT sm.story_id
FROM news.story_members sm
JOIN news.documents d ON d.document_id = sm.document_id
JOIN news.stories s ON s.story_id = sm.story_id
WHERE s.status = 'active'
  AND s.collection = $1
  AND d.collection = $1
  AND d.source = $2
  AND d.source_item_id = $3
ORDER BY sm.matched_at DESC
LIMIT 1
`
	var storyID int64
	err := tx.QueryRow(ctx, q, collection, source, sourceItemID).Scan(&storyID)
	if err != nil {
		if err == db.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("find exact_source_id story: %w", err)
	}
	return storyID, true, nil
}

func findExactContentHashStoryTx(ctx context.Context, tx db.Tx, collection string, contentHash []byte) (int64, bool, error) {
	if len(contentHash) == 0 {
		return 0, false, nil
	}
	const q = `
SELECT sm.story_id
FROM news.story_members sm
JOIN news.documents d ON d.document_id = sm.document_id
JOIN news.stories s ON s.story_id = sm.story_id
WHERE s.status = 'active'
  AND s.collection = $1
  AND d.collection = $1
  AND d.content_hash = $2
ORDER BY sm.matched_at DESC
LIMIT 1
`
	var storyID int64
	err := tx.QueryRow(ctx, q, collection, contentHash).Scan(&storyID)
	if err != nil {
		if err == db.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("find exact_content_hash story: %w", err)
	}
	return storyID, true, nil
}

func findSemanticCandidatesTx(
	ctx context.Context,
	tx db.Tx,
	embeddingVector string,
	collection string,
	modelName string,
	modelVersion string,
	lookbackCutoff time.Time,
	limit int,
) ([]semanticCandidate, error) {
	if limit <= 0 {
		limit = defaultSemanticCandidateLimit
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL hnsw.ef_search = %d", defaultSemanticSearchEF)); err != nil {
		return nil, fmt.Errorf("set hnsw.ef_search: %w", err)
	}

	const q = `
SELECT
	s.story_id,
	COALESCE(rd.normalized_title, s.canonical_title) AS candidate_title,
	s.last_seen_at,
	(1 - (de.embedding <=> $1::vector))::DOUBLE PRECISION AS cosine
FROM news.stories s
LEFT JOIN news.documents rd ON rd.document_id = s.representative_document_id
JOIN news.document_embeddings de ON de.document_id = s.representative_document_id
WHERE s.status = 'active'
  AND s.collection = $2
  AND de.model_name = $3
  AND de.model_version = $4
  AND s.last_seen_at >= $5
ORDER BY de.embedding <=> $1::vector ASC
LIMIT $6
`

	rows, err := tx.Query(ctx, q, embeddingVector, collection, modelName, modelVersion, lookbackCutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("query semantic candidates: %w", err)
	}
	defer rows.Close()

	candidates := make([]semanticCandidate, 0, limit)
	for rows.Next() {
		var (
			c         semanticCandidate
			titleText string
		)
		if err := rows.Scan(&c.StoryID, &titleText, &c.LastSeenAt, &c.Cosine); err != nil {
			return nil, fmt.Errorf("scan semantic candidate: %w", err)
		}
		c.Title = titleText
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate semantic candidates: %w", err)
	}
	return candidates, nil
}

func findLexicalAutoMergeTx(
	ctx context.Context,
	tx db.Tx,
	doc pendingDocument,
	lookbackCutoff time.Time,
) (lexicalAutoMergeMatch, bool, error) {
	const q = `
SELECT
	s.story_id,
	COALESCE(rd.normalized_title, s.canonical_title) AS candidate_title,
	s.last_seen_at,
	s.source_count,
	s.item_count,
	s.canonical_url,
	rd.title_simhash
FROM news.stories s
LEFT JOIN news.documents rd ON rd.document_id = s.representative_document_id
WHERE s.status = 'active'
  AND s.collection = $3
  AND s.last_seen_at >= $2
ORDER BY s.last_seen_at DESC
LIMIT $1
`

	rows, err := tx.Query(ctx, q, storyCandidateLimit, lookbackCutoff, doc.Collection)
	if err != nil {
		return lexicalAutoMergeMatch{}, false, fmt.Errorf("query lexical candidates: %w", err)
	}
	defer rows.Close()

	bestSimhashDistance := 65
	var bestSimhash lexicalAutoMergeMatch
	var hasSimhash bool

	var bestOverlap lexicalAutoMergeMatch
	var hasOverlap bool

	for rows.Next() {
		var c storyCandidate
		var canonicalURL *string
		var titleSimhash *int64
		if err := rows.Scan(
			&c.StoryID,
			&c.Title,
			&c.LastSeenAt,
			&c.SourceCount,
			&c.ItemCount,
			&canonicalURL,
			&titleSimhash,
		); err != nil {
			return lexicalAutoMergeMatch{}, false, fmt.Errorf("scan lexical candidate: %w", err)
		}
		c.CanonicalURL = canonicalURL
		c.TitleSimhash = titleSimhash

		dateConsistency := computeDateConsistency(doc.PublishedAt, c.LastSeenAt)

		if distance, ok := titleSimhashDistance(doc.TitleSimhash, c.TitleSimhash); ok && distance <= defaultLexicalSimhashMaxDistance {
			score := 1 - (float64(distance) / 64.0)
			if !hasSimhash || distance < bestSimhashDistance || (distance == bestSimhashDistance && c.LastSeenAt.After(bestSimhash.Candidate.LastSeenAt)) {
				distanceCopy := distance
				bestSimhash = lexicalAutoMergeMatch{
					Candidate:       c,
					Signal:          "lexical_simhash",
					MatchScore:      score,
					TitleOverlap:    titleTokenJaccard(doc.NormalizedTitle, c.Title),
					DateConsistency: dateConsistency,
					CompositeScore:  score,
					SimhashDistance: &distanceCopy,
				}
				bestSimhashDistance = distance
				hasSimhash = true
			}
		}

		overlap := titleTrigramJaccard(doc.NormalizedTitle, c.Title)
		if overlap < defaultLexicalTrigramThreshold {
			continue
		}
		if !isWithinDateWindow(doc.PublishedAt, c.LastSeenAt, defaultLexicalTrigramDateWindow) {
			continue
		}

		composite := (0.8 * overlap) + (0.2 * dateConsistency)
		if !hasOverlap || composite > bestOverlap.CompositeScore {
			bestOverlap = lexicalAutoMergeMatch{
				Candidate:       c,
				Signal:          "lexical_overlap",
				MatchScore:      composite,
				TitleOverlap:    overlap,
				DateConsistency: dateConsistency,
				CompositeScore:  composite,
			}
			hasOverlap = true
		}
	}

	if err := rows.Err(); err != nil {
		return lexicalAutoMergeMatch{}, false, fmt.Errorf("iterate lexical candidates: %w", err)
	}

	if hasSimhash {
		return bestSimhash, true, nil
	}
	if hasOverlap {
		return bestOverlap, true, nil
	}
	return lexicalAutoMergeMatch{}, false, nil
}

func createStoryTx(
	ctx context.Context,
	tx db.Tx,
	doc pendingDocument,
	documentSeenAt time.Time,
	now time.Time,
) (int64, error) {
	const q = `
INSERT INTO news.stories (
	canonical_title,
	canonical_url,
	canonical_url_hash,
	collection,
	representative_document_id,
	first_seen_at,
	last_seen_at,
	source_count,
	item_count,
	status,
	created_at,
	updated_at
)
VALUES (
	$1,
	$2,
	$3,
	$4,
	$5,
	$6,
	$6,
	1,
	1,
	'active',
	$7,
	$7
)
RETURNING story_id
`
	var storyID int64
	err := tx.QueryRow(
		ctx,
		q,
		doc.NormalizedTitle,
		doc.CanonicalURL,
		nullableBytes(doc.CanonicalURLHash),
		doc.Collection,
		doc.DocumentID,
		documentSeenAt,
		now,
	).Scan(&storyID)
	if err != nil {
		return 0, fmt.Errorf("insert story for document_id=%d: %w", doc.DocumentID, err)
	}
	return storyID, nil
}

func upsertStoryMemberTx(
	ctx context.Context,
	tx db.Tx,
	storyID int64,
	documentID int64,
	matchType string,
	matchScore *float64,
	matchDetails map[string]any,
	now time.Time,
) (bool, error) {
	const q = `
INSERT INTO news.story_members (
	story_id,
	document_id,
	match_type,
	match_score,
	match_details,
	matched_at
)
VALUES ($1, $2, $3, $4, $5::jsonb, $6)
ON CONFLICT (document_id) DO NOTHING
`

	detailsJSON, err := json.Marshal(matchDetails)
	if err != nil {
		return false, fmt.Errorf("marshal story member details: %w", err)
	}

	commandTag, err := tx.Exec(ctx, q, storyID, documentID, matchType, matchScore, string(detailsJSON), now)
	if err != nil {
		return false, fmt.Errorf("insert story_member story_id=%d document_id=%d: %w", storyID, documentID, err)
	}
	return commandTag.RowsAffected() == 1, nil
}

func refreshStoryAggregateTx(
	ctx context.Context,
	tx db.Tx,
	storyID int64,
	documentID int64,
	documentTitle string,
	documentCanonicalURL *string,
	documentCanonicalURLHash []byte,
	documentSeenAt time.Time,
	now time.Time,
) error {
	const q = `
UPDATE news.stories s
SET
	first_seen_at = LEAST(s.first_seen_at, $2),
	last_seen_at = GREATEST(s.last_seen_at, $3),
	source_count = agg.source_count,
	item_count = agg.item_count,
	representative_document_id = COALESCE(s.representative_document_id, $4),
	canonical_title = CASE WHEN s.representative_document_id IS NULL THEN $5 ELSE s.canonical_title END,
	canonical_url = CASE WHEN s.representative_document_id IS NULL THEN $6 ELSE s.canonical_url END,
	canonical_url_hash = CASE WHEN s.representative_document_id IS NULL THEN $7 ELSE s.canonical_url_hash END,
	updated_at = $1
FROM (
	SELECT
		sm.story_id,
		COUNT(*)::INT AS item_count,
		COUNT(DISTINCT d.source)::INT AS source_count
	FROM news.story_members sm
	JOIN news.documents d ON d.document_id = sm.document_id
	WHERE sm.story_id = $8
	GROUP BY sm.story_id
) agg
WHERE s.story_id = agg.story_id
`
	_, err := tx.Exec(
		ctx,
		q,
		now,
		documentSeenAt,
		documentSeenAt,
		documentID,
		documentTitle,
		documentCanonicalURL,
		nullableBytes(documentCanonicalURLHash),
		storyID,
	)
	if err != nil {
		return fmt.Errorf("refresh story aggregate story_id=%d: %w", storyID, err)
	}
	return nil
}

type dedupEventRecord struct {
	DocumentID            int64
	Decision              string
	ChosenStoryID         *int64
	BestCandidateStoryID  *int64
	BestCosine            *float64
	TitleOverlap          *float64
	EntityDateConsistency *float64
	CompositeScore        *float64
	ExactSignal           *string
	CreatedAt             time.Time
}

func insertDedupEventTx(ctx context.Context, tx db.Tx, record dedupEventRecord) error {
	const q = `
INSERT INTO news.dedup_events (
	document_id,
	decision,
	chosen_story_id,
	best_candidate_story_id,
	best_cosine,
	title_overlap,
	entity_date_consistency,
	composite_score,
	exact_signal,
	created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (document_id) DO NOTHING
`
	_, err := tx.Exec(
		ctx,
		q,
		record.DocumentID,
		record.Decision,
		record.ChosenStoryID,
		record.BestCandidateStoryID,
		record.BestCosine,
		record.TitleOverlap,
		record.EntityDateConsistency,
		record.CompositeScore,
		record.ExactSignal,
		record.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert dedup_event document_id=%d: %w", record.DocumentID, err)
	}
	return nil
}

func normalizeText(input string) string {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if trimmed == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(trimmed))
	lastSpace := false
	for _, r := range trimmed {
		if unicode.IsSpace(r) {
			if !lastSpace {
				b.WriteRune(' ')
				lastSpace = true
			}
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
		lastSpace = false
	}
	return strings.TrimSpace(b.String())
}

func extractCollectionFromMetadata(metadata map[string]any) string {
	if len(metadata) == 0 {
		return ""
	}
	raw, ok := metadata["collection"]
	if !ok {
		return ""
	}
	label, ok := raw.(string)
	if !ok {
		return ""
	}
	return normalizeCollectionLabel(label)
}

func normalizeCollectionLabel(raw string) string {
	return strings.TrimSpace(strings.ToLower(raw))
}

func normalizeURL(raw string) (canonical string, host string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", ""
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", ""
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Hostname())
	if port := parsed.Port(); port != "" {
		defaultPort := (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443")
		if !defaultPort {
			parsed.Host = parsed.Host + ":" + port
		}
	}

	parsed.Fragment = ""
	path := strings.TrimSpace(parsed.EscapedPath())
	if path == "" {
		path = "/"
	}
	path = strings.ReplaceAll(path, "//", "/")
	if strings.HasSuffix(path, "/") && path != "/" {
		path = strings.TrimSuffix(path, "/")
	}
	parsed.Path = path
	parsed.RawPath = ""

	q := parsed.Query()
	for key := range q {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") {
			q.Del(key)
			continue
		}
		if _, ok := trackingQueryKeys[lower]; ok {
			q.Del(key)
		}
	}
	if len(q) > 0 {
		keys := make([]string, 0, len(q))
		for key := range q {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		reordered := url.Values{}
		for _, key := range keys {
			values := q[key]
			sort.Strings(values)
			for _, value := range values {
				reordered.Add(key, value)
			}
		}
		parsed.RawQuery = reordered.Encode()
	} else {
		parsed.RawQuery = ""
	}

	return parsed.String(), parsed.Hostname()
}

func countTokens(text string) int {
	if strings.TrimSpace(text) == "" {
		return 0
	}
	return len(strings.Fields(text))
}

func simhash64(text string) (uint64, bool) {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return 0, false
	}

	var bitWeights [64]int
	for _, token := range tokens {
		h := hashToken64(token)
		for bit := 0; bit < 64; bit++ {
			mask := uint64(1) << bit
			if h&mask != 0 {
				bitWeights[bit]++
			} else {
				bitWeights[bit]--
			}
		}
	}

	var result uint64
	for bit := 0; bit < 64; bit++ {
		if bitWeights[bit] > 0 {
			result |= uint64(1) << bit
		}
	}
	return result, true
}

func tokenize(text string) []string {
	normalized := normalizeText(text)
	if normalized == "" {
		return nil
	}

	parts := strings.FieldsFunc(normalized, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		tokens = append(tokens, p)
	}
	return tokens
}

func hashToken64(token string) uint64 {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(token))
	return hasher.Sum64()
}

func titleTokenJaccard(left, right string) float64 {
	leftSet := tokenSet(left)
	rightSet := tokenSet(right)
	if len(leftSet) == 0 || len(rightSet) == 0 {
		return 0
	}

	intersection := 0
	for token := range leftSet {
		if _, ok := rightSet[token]; ok {
			intersection++
		}
	}
	if intersection == 0 {
		return 0
	}

	union := len(leftSet) + len(rightSet) - intersection
	if union <= 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func tokenSet(text string) map[string]struct{} {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		set[token] = struct{}{}
	}
	return set
}

func titleTrigramJaccard(left, right string) float64 {
	leftSet := trigramSet(left)
	rightSet := trigramSet(right)
	if len(leftSet) == 0 || len(rightSet) == 0 {
		return 0
	}

	intersection := 0
	for token := range leftSet {
		if _, ok := rightSet[token]; ok {
			intersection++
		}
	}
	if intersection == 0 {
		return 0
	}

	union := len(leftSet) + len(rightSet) - intersection
	if union <= 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func trigramSet(text string) map[string]struct{} {
	normalized := normalizeText(text)
	if normalized == "" {
		return nil
	}

	runes := []rune(normalized)
	if len(runes) < 3 {
		return map[string]struct{}{string(runes): {}}
	}

	set := make(map[string]struct{}, len(runes)-2)
	for i := 0; i <= len(runes)-3; i++ {
		set[string(runes[i:i+3])] = struct{}{}
	}
	return set
}

func titleSimhashDistance(left, right *int64) (int, bool) {
	if left == nil || right == nil {
		return 0, false
	}
	return bits.OnesCount64(uint64(*left) ^ uint64(*right)), true
}

func isWithinDateWindow(documentPublishedAt *time.Time, storyLastSeen time.Time, window time.Duration) bool {
	if documentPublishedAt == nil || documentPublishedAt.IsZero() {
		return false
	}
	diff := math.Abs(documentPublishedAt.UTC().Sub(storyLastSeen.UTC()).Hours())
	return diff <= window.Hours()
}

func computeDateConsistency(documentPublishedAt *time.Time, storyLastSeen time.Time) float64 {
	if documentPublishedAt == nil || documentPublishedAt.IsZero() {
		return 0.5
	}
	diff := math.Abs(documentPublishedAt.UTC().Sub(storyLastSeen.UTC()).Hours())
	switch {
	case diff <= 48:
		return 1
	case diff <= 7*24:
		return 0.6
	default:
		return 0
	}
}

func semanticCompositeScore(cosine, titleOverlap, dateConsistency float64) float64 {
	score := (semanticCompositeCosineWeight * cosine) +
		(semanticCompositeTitleWeight * titleOverlap) +
		(semanticCompositeDateWeight * dateConsistency)
	switch {
	case score < 0:
		return 0
	case score > 1:
		return 1
	default:
		return score
	}
}

func shouldAutoMergeSemantic(cosine, titleOverlap float64) bool {
	if cosine >= defaultSemanticOverrideCosine {
		return true
	}
	return cosine >= defaultSemanticAutoMergeCosine && titleOverlap >= defaultSemanticTitleOverlapFloor
}

func shouldMarkSemanticGrayZone(cosine float64) bool {
	return cosine >= defaultSemanticGrayZoneMinCosine && cosine < defaultSemanticAutoMergeCosine
}

func nullableBytes(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	copyValue := make([]byte, len(value))
	copy(copyValue, value)
	return copyValue
}

func floatPtr(v float64) *float64 {
	p := new(float64)
	*p = v
	return p
}
