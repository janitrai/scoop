package translation

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"horse.fit/scoop/internal/db"
	"horse.fit/scoop/internal/reader"
)

const (
	SourceTypeStoryTitle    = "story_title"
	SourceTypeStorySummary  = "story_summary"
	SourceTypeArticleTitle  = "article_title"
	SourceTypeArticleText   = "article_text"
	ContentOriginNormalized = "normalized"
	ContentOriginReader     = "reader"

	// Keep reader-fetched body translations bounded by truncating at rune boundaries.
	articleReaderTranslationMaxChars = 6000
)

var (
	ErrStoryNotFound   = errors.New("story not found")
	ErrArticleNotFound = errors.New("article not found")
)

// RunOptions controls translation execution.
type RunOptions struct {
	TargetLang string
	Provider   string
	Force      bool
	DryRun     bool
}

// CollectionRunOptions controls collection-level translation execution.
type CollectionRunOptions struct {
	RunOptions
	Progress func(CollectionProgress)
}

// CollectionProgress reports story-level progress for collection translations.
type CollectionProgress struct {
	Current   int
	Total     int
	StoryID   int64
	StoryUUID string
}

// RunStats reports translation execution counters.
type RunStats struct {
	Total      int `json:"total"`
	Translated int `json:"translated"`
	Cached     int `json:"cached"`
	Skipped    int `json:"skipped"`
}

// CachedTranslation is a cached translation row enriched for API output.
type CachedTranslation struct {
	TranslationUUID string    `json:"translation_uuid"`
	SourceType      string    `json:"source_type"`
	SourceID        int64     `json:"source_id"`
	SourceUUID      *string   `json:"source_uuid,omitempty"`
	SourceLang      string    `json:"source_lang"`
	TargetLang      string    `json:"target_lang"`
	OriginalText    string    `json:"original_text"`
	TranslatedText  string    `json:"translated_text"`
	ProviderName    string    `json:"provider_name"`
	ModelName       *string   `json:"model_name,omitempty"`
	LatencyMS       *int      `json:"latency_ms,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// Manager coordinates provider calls and persistent translation caching.
type Manager struct {
	pool     *db.Pool
	registry *Registry
}

func NewManager(pool *db.Pool, registry *Registry) *Manager {
	return &Manager{pool: pool, registry: registry}
}

func (m *Manager) DefaultProvider() string {
	if m == nil || m.registry == nil {
		return ""
	}
	return m.registry.DefaultProvider()
}

func (m *Manager) TranslateStoryByUUID(ctx context.Context, storyUUID string, opts RunOptions) (RunStats, error) {
	if m == nil || m.pool == nil {
		return RunStats{}, fmt.Errorf("translation manager is not initialized")
	}

	story, err := m.fetchStoryByUUID(ctx, storyUUID)
	if err != nil {
		return RunStats{}, err
	}
	return m.translateStory(ctx, story, opts)
}

func (m *Manager) TranslateArticleByUUID(ctx context.Context, articleUUID string, opts RunOptions) (RunStats, error) {
	if m == nil || m.pool == nil {
		return RunStats{}, fmt.Errorf("translation manager is not initialized")
	}

	targetLang := normalizeLangCode(opts.TargetLang)
	if targetLang == "" {
		return RunStats{}, fmt.Errorf("target language is required")
	}
	opts.TargetLang = targetLang

	article, err := m.fetchArticleByUUID(ctx, articleUUID)
	if err != nil {
		return RunStats{}, err
	}

	tasks := make([]translationTask, 0, 2)
	if strings.TrimSpace(article.Title) != "" {
		tasks = append(tasks, translationTask{
			SourceType:    SourceTypeArticleTitle,
			SourceID:      article.ArticleID,
			SourceLang:    article.SourceLang,
			OriginalText:  article.Title,
			ContentOrigin: ContentOriginNormalized,
		})
	}
	if strings.TrimSpace(article.Text) != "" {
		tasks = append(tasks, translationTask{
			SourceType:    SourceTypeArticleText,
			SourceID:      article.ArticleID,
			SourceLang:    article.SourceLang,
			OriginalText:  article.Text,
			ContentOrigin: article.TextOrigin,
		})
	}

	return m.runTasks(ctx, tasks, opts)
}

func (m *Manager) TranslateCollection(ctx context.Context, collection string, opts CollectionRunOptions) (RunStats, error) {
	if m == nil || m.pool == nil {
		return RunStats{}, fmt.Errorf("translation manager is not initialized")
	}

	stories, err := m.listStoriesByCollection(ctx, collection)
	if err != nil {
		return RunStats{}, err
	}

	total := RunStats{}
	for idx, story := range stories {
		if opts.Progress != nil {
			opts.Progress(CollectionProgress{
				Current:   idx + 1,
				Total:     len(stories),
				StoryID:   story.StoryID,
				StoryUUID: story.StoryUUID,
			})
		}

		stats, err := m.translateStory(ctx, story, opts.RunOptions)
		if err != nil {
			return total, err
		}

		total.Total += stats.Total
		total.Translated += stats.Translated
		total.Cached += stats.Cached
		total.Skipped += stats.Skipped
	}

	return total, nil
}

func (m *Manager) ListStoryTranslationsByUUID(ctx context.Context, storyUUID string) ([]CachedTranslation, error) {
	if m == nil || m.pool == nil {
		return nil, fmt.Errorf("translation manager is not initialized")
	}

	story, err := m.fetchStoryByUUID(ctx, storyUUID)
	if err != nil {
		return nil, err
	}

	const q = `
SELECT
	r.translation_result_uuid::text,
	ts.source_type,
	ts.source_id,
	CASE
		WHEN ts.source_type = 'story_title' THEN s.story_uuid::text
		WHEN ts.source_type IN ('article_title', 'article_text') THEN a.article_uuid::text
		ELSE NULL
	END AS source_uuid,
	ts.source_lang,
	r.target_lang,
	ts.original_text,
	r.translated_text,
	r.provider_name,
	r.model_name,
	r.latency_ms,
	r.created_at
FROM news.translation_sources ts
JOIN news.translation_results r
	ON r.translation_source_id = ts.translation_source_id
LEFT JOIN news.stories s
	ON ts.source_type = 'story_title'
	AND s.story_id = ts.source_id
LEFT JOIN news.articles a
	ON ts.source_type IN ('article_title', 'article_text')
	AND a.article_id = ts.source_id
WHERE (ts.source_type = 'story_title' AND ts.source_id = $1)
   OR (
		ts.source_type IN ('article_title', 'article_text')
		AND ts.source_id IN (
			SELECT sa.article_id
			FROM news.story_articles sa
			WHERE sa.story_id = $1
		)
	)
ORDER BY r.target_lang, ts.source_type, ts.source_id, ts.captured_at DESC, r.created_at DESC
	`

	rows, err := m.pool.Query(ctx, q, story.StoryID)
	if err != nil {
		return nil, fmt.Errorf("query story translations: %w", err)
	}
	defer rows.Close()

	items := make([]CachedTranslation, 0, 32)
	for rows.Next() {
		var row CachedTranslation
		if err := rows.Scan(
			&row.TranslationUUID,
			&row.SourceType,
			&row.SourceID,
			&row.SourceUUID,
			&row.SourceLang,
			&row.TargetLang,
			&row.OriginalText,
			&row.TranslatedText,
			&row.ProviderName,
			&row.ModelName,
			&row.LatencyMS,
			&row.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan story translation row: %w", err)
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate story translations: %w", err)
	}

	return items, nil
}

func (m *Manager) translateStory(ctx context.Context, story storyTranslationTarget, opts RunOptions) (RunStats, error) {
	targetLang := normalizeLangCode(opts.TargetLang)
	if targetLang == "" {
		return RunStats{}, fmt.Errorf("target language is required")
	}
	opts.TargetLang = targetLang

	articles, err := m.fetchStoryArticles(ctx, story.StoryID)
	if err != nil {
		return RunStats{}, err
	}

	tasks := make([]translationTask, 0, 1+(2*len(articles)))
	if strings.TrimSpace(story.Title) != "" {
		tasks = append(tasks, translationTask{
			SourceType:    SourceTypeStoryTitle,
			SourceID:      story.StoryID,
			SourceLang:    story.SourceLang,
			OriginalText:  story.Title,
			ContentOrigin: ContentOriginNormalized,
		})
	}

	for _, article := range articles {
		if strings.TrimSpace(article.Title) != "" {
			tasks = append(tasks, translationTask{
				SourceType:    SourceTypeArticleTitle,
				SourceID:      article.ArticleID,
				SourceLang:    article.SourceLang,
				OriginalText:  article.Title,
				ContentOrigin: ContentOriginNormalized,
			})
		}
		if strings.TrimSpace(article.Text) != "" {
			tasks = append(tasks, translationTask{
				SourceType:    SourceTypeArticleText,
				SourceID:      article.ArticleID,
				SourceLang:    article.SourceLang,
				OriginalText:  article.Text,
				ContentOrigin: article.TextOrigin,
			})
		}
	}

	return m.runTasks(ctx, tasks, opts)
}

func (m *Manager) runTasks(ctx context.Context, tasks []translationTask, opts RunOptions) (RunStats, error) {
	targetLang := normalizeLangCode(opts.TargetLang)
	if targetLang == "" {
		return RunStats{}, fmt.Errorf("target language is required")
	}

	provider, err := m.resolveProvider(opts.Provider)
	if err != nil {
		return RunStats{}, err
	}
	providerName := provider.Name()
	modelName := modelNameFromProvider(provider)

	stats := RunStats{}
	for _, task := range tasks {
		stats.Total++

		originalText := strings.TrimSpace(task.OriginalText)
		if originalText == "" {
			stats.Skipped++
			continue
		}

		contentHash := hashTranslationSourceText(originalText)
		sourceLang := normalizeLangCode(task.SourceLang)
		if sourceLang == "" {
			sourceLang = "und"
		}

		translationSourceID, err := m.upsertTranslationSource(ctx, upsertTranslationSourceInput{
			SourceType:    task.SourceType,
			SourceID:      task.SourceID,
			SourceLang:    sourceLang,
			ContentHash:   contentHash,
			OriginalText:  originalText,
			ContentOrigin: normalizeContentOrigin(task.ContentOrigin),
		})
		if err != nil {
			return stats, err
		}

		cached, err := m.lookupCachedTranslation(ctx, translationSourceID, targetLang)
		if err != nil {
			return stats, err
		}
		if cached != nil && !opts.Force {
			stats.Cached++
			continue
		}

		if opts.DryRun {
			stats.Skipped++
			continue
		}

		resp, err := provider.Translate(ctx, TranslateRequest{
			Text:       originalText,
			SourceLang: sourceLang,
			TargetLang: targetLang,
		})
		if err != nil {
			return stats, fmt.Errorf("translate %s source_id=%d: %w", task.SourceType, task.SourceID, err)
		}

		translatedText := strings.TrimSpace(resp.Text)
		if translatedText == "" {
			return stats, fmt.Errorf("translate %s source_id=%d: empty translation", task.SourceType, task.SourceID)
		}

		resolvedSourceLang := normalizeLangCode(resp.SourceLang)
		if resolvedSourceLang == "" {
			resolvedSourceLang = normalizeLangCode(task.SourceLang)
		}
		if resolvedSourceLang == "" {
			resolvedSourceLang = "und"
		}

		resolvedTargetLang := normalizeLangCode(resp.TargetLang)
		if resolvedTargetLang == "" {
			resolvedTargetLang = targetLang
		}

		if resolvedSourceLang != sourceLang {
			translationSourceID, err = m.upsertTranslationSource(ctx, upsertTranslationSourceInput{
				SourceType:    task.SourceType,
				SourceID:      task.SourceID,
				SourceLang:    resolvedSourceLang,
				ContentHash:   contentHash,
				OriginalText:  originalText,
				ContentOrigin: normalizeContentOrigin(task.ContentOrigin),
			})
			if err != nil {
				return stats, err
			}
		}

		resolvedProviderName := strings.TrimSpace(resp.ProviderName)
		if resolvedProviderName == "" {
			resolvedProviderName = providerName
		}

		latencyMS := int(resp.LatencyMs)
		if latencyMS < 0 {
			latencyMS = 0
		}

		if err := m.upsertTranslationResult(ctx, upsertTranslationResultInput{
			TranslationSourceID: translationSourceID,
			TargetLang:          resolvedTargetLang,
			TranslatedText:      translatedText,
			ProviderName:        resolvedProviderName,
			ModelName:           modelName,
			LatencyMS:           &latencyMS,
		}); err != nil {
			return stats, err
		}

		stats.Translated++
	}

	return stats, nil
}

func (m *Manager) resolveProvider(requested string) (Provider, error) {
	if m == nil || m.registry == nil {
		return nil, fmt.Errorf("translation provider registry is not initialized")
	}
	return m.registry.Provider(requested)
}

func (m *Manager) fetchStoryByUUID(ctx context.Context, storyUUID string) (storyTranslationTarget, error) {
	const q = `
SELECT
	s.story_id,
	s.story_uuid::text,
	s.canonical_title,
	COALESCE(rep.normalized_language, 'und') AS source_lang
FROM news.stories s
LEFT JOIN news.articles rep
	ON rep.article_id = s.representative_article_id
	AND rep.deleted_at IS NULL
WHERE s.story_uuid = $1::uuid
  AND s.deleted_at IS NULL
LIMIT 1
`

	var row storyTranslationTarget
	err := m.pool.QueryRow(ctx, q, strings.TrimSpace(storyUUID)).Scan(
		&row.StoryID,
		&row.StoryUUID,
		&row.Title,
		&row.SourceLang,
	)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return storyTranslationTarget{}, ErrStoryNotFound
		}
		return storyTranslationTarget{}, fmt.Errorf("query story: %w", err)
	}
	return row, nil
}

func (m *Manager) listStoriesByCollection(ctx context.Context, collection string) ([]storyTranslationTarget, error) {
	const q = `
SELECT
	s.story_id,
	s.story_uuid::text,
	s.canonical_title,
	COALESCE(rep.normalized_language, 'und') AS source_lang
FROM news.stories s
LEFT JOIN news.articles rep
	ON rep.article_id = s.representative_article_id
	AND rep.deleted_at IS NULL
WHERE s.deleted_at IS NULL
  AND ($1 = '' OR s.collection = $1)
ORDER BY s.last_seen_at DESC, s.story_id DESC
`

	rows, err := m.pool.Query(ctx, q, normalizeCollection(collection))
	if err != nil {
		return nil, fmt.Errorf("query collection stories: %w", err)
	}
	defer rows.Close()

	items := make([]storyTranslationTarget, 0, 64)
	for rows.Next() {
		var row storyTranslationTarget
		if err := rows.Scan(&row.StoryID, &row.StoryUUID, &row.Title, &row.SourceLang); err != nil {
			return nil, fmt.Errorf("scan collection story row: %w", err)
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate collection stories: %w", err)
	}

	return items, nil
}

func (m *Manager) fetchStoryArticles(ctx context.Context, storyID int64) ([]articleTranslationTarget, error) {
	const q = `
SELECT
	a.article_id,
	a.article_uuid::text,
	a.normalized_title,
	a.normalized_text,
	a.normalized_language,
	a.canonical_url
FROM news.story_articles sa
JOIN news.articles a
	ON a.article_id = sa.article_id
	AND a.deleted_at IS NULL
WHERE sa.story_id = $1
ORDER BY sa.matched_at DESC, a.article_id DESC
`

	rows, err := m.pool.Query(ctx, q, storyID)
	if err != nil {
		return nil, fmt.Errorf("query story articles: %w", err)
	}
	defer rows.Close()

	items := make([]articleTranslationTarget, 0, 8)
	for rows.Next() {
		var row articleTranslationTarget
		if err := rows.Scan(
			&row.ArticleID,
			&row.ArticleUUID,
			&row.Title,
			&row.Text,
			&row.SourceLang,
			&row.CanonicalURL,
		); err != nil {
			return nil, fmt.Errorf("scan story article row: %w", err)
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate story articles: %w", err)
	}
	rows.Close()

	for i := range items {
		if err := m.hydrateArticleTextForTranslation(ctx, &items[i]); err != nil {
			return nil, err
		}
	}

	return items, nil
}

func (m *Manager) fetchArticleByUUID(ctx context.Context, articleUUID string) (articleTranslationTarget, error) {
	const q = `
SELECT
	a.article_id,
	a.article_uuid::text,
	a.normalized_title,
	a.normalized_text,
	a.normalized_language,
	a.canonical_url
FROM news.articles a
WHERE a.article_uuid = $1::uuid
  AND a.deleted_at IS NULL
LIMIT 1
`

	var row articleTranslationTarget
	err := m.pool.QueryRow(ctx, q, strings.TrimSpace(articleUUID)).Scan(
		&row.ArticleID,
		&row.ArticleUUID,
		&row.Title,
		&row.Text,
		&row.SourceLang,
		&row.CanonicalURL,
	)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return articleTranslationTarget{}, ErrArticleNotFound
		}
		return articleTranslationTarget{}, fmt.Errorf("query article: %w", err)
	}

	if err := m.hydrateArticleTextForTranslation(ctx, &row); err != nil {
		return articleTranslationTarget{}, err
	}

	return row, nil
}

func (m *Manager) hydrateArticleTextForTranslation(
	ctx context.Context,
	article *articleTranslationTarget,
) error {
	if article == nil {
		return nil
	}

	article.Title = strings.TrimSpace(article.Title)
	article.Text = strings.TrimSpace(article.Text)
	article.TextOrigin = ContentOriginNormalized

	if article.Text != "" {
		return nil
	}

	canonicalURL := ""
	if article.CanonicalURL != nil {
		canonicalURL = strings.TrimSpace(*article.CanonicalURL)
	}
	if canonicalURL == "" {
		return nil
	}

	readerText, err := reader.FetchText(ctx, canonicalURL, article.Title)
	if err != nil {
		// Reader fetch is best-effort; keep the prior behavior of skipping empty bodies.
		return nil
	}

	clipped, _ := reader.TruncateText(readerText, articleReaderTranslationMaxChars)
	article.Text = strings.TrimSpace(clipped)
	if article.Text != "" {
		article.TextOrigin = ContentOriginReader
	}
	return nil
}

func (m *Manager) lookupCachedTranslation(
	ctx context.Context,
	translationSourceID int64,
	targetLang string,
) (*CachedTranslation, error) {
	const q = `
SELECT
	r.translation_result_uuid::text,
	ts.source_type,
	ts.source_id,
	ts.source_lang,
	r.target_lang,
	ts.original_text,
	r.translated_text,
	r.provider_name,
	r.model_name,
	r.latency_ms,
	r.created_at
FROM news.translation_results r
JOIN news.translation_sources ts
	ON ts.translation_source_id = r.translation_source_id
WHERE r.translation_source_id = $1
	  AND r.target_lang = $2
LIMIT 1
	`

	var row CachedTranslation
	err := m.pool.QueryRow(ctx, q, translationSourceID, targetLang).Scan(
		&row.TranslationUUID,
		&row.SourceType,
		&row.SourceID,
		&row.SourceLang,
		&row.TargetLang,
		&row.OriginalText,
		&row.TranslatedText,
		&row.ProviderName,
		&row.ModelName,
		&row.LatencyMS,
		&row.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query translation cache: %w", err)
	}
	return &row, nil
}

type upsertTranslationSourceInput struct {
	SourceType    string
	SourceID      int64
	SourceLang    string
	ContentHash   []byte
	OriginalText  string
	ContentOrigin string
}

func (m *Manager) upsertTranslationSource(ctx context.Context, row upsertTranslationSourceInput) (int64, error) {
	const q = `
INSERT INTO news.translation_sources (
	source_type,
	source_id,
	source_lang,
	content_hash,
	original_text,
	content_origin,
	captured_at
)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (source_type, source_id, content_hash)
DO UPDATE SET
	source_lang = EXCLUDED.source_lang,
	original_text = EXCLUDED.original_text,
	content_origin = EXCLUDED.content_origin,
	captured_at = now()
RETURNING translation_source_id
	`

	var translationSourceID int64
	if err := m.pool.QueryRow(
		ctx,
		q,
		row.SourceType,
		row.SourceID,
		row.SourceLang,
		row.ContentHash,
		row.OriginalText,
		row.ContentOrigin,
	).Scan(&translationSourceID); err != nil {
		return 0, fmt.Errorf("upsert translation source: %w", err)
	}
	return translationSourceID, nil
}

type upsertTranslationResultInput struct {
	TranslationSourceID int64
	TargetLang          string
	TranslatedText      string
	ProviderName        string
	ModelName           *string
	LatencyMS           *int
}

func (m *Manager) upsertTranslationResult(ctx context.Context, row upsertTranslationResultInput) error {
	const q = `
INSERT INTO news.translation_results (
	translation_source_id,
	target_lang,
	translated_text,
	provider_name,
	model_name,
	latency_ms
)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (translation_source_id, target_lang)
DO UPDATE SET
	translated_text = EXCLUDED.translated_text,
	provider_name = EXCLUDED.provider_name,
	model_name = EXCLUDED.model_name,
	latency_ms = EXCLUDED.latency_ms,
	created_at = now()
	`

	if _, err := m.pool.Exec(
		ctx,
		q,
		row.TranslationSourceID,
		row.TargetLang,
		row.TranslatedText,
		row.ProviderName,
		row.ModelName,
		row.LatencyMS,
	); err != nil {
		return fmt.Errorf("upsert translation result: %w", err)
	}
	return nil
}

type translationTask struct {
	SourceType    string
	SourceID      int64
	SourceLang    string
	OriginalText  string
	ContentOrigin string
}

type storyTranslationTarget struct {
	StoryID    int64
	StoryUUID  string
	Title      string
	SourceLang string
}

type articleTranslationTarget struct {
	ArticleID    int64
	ArticleUUID  string
	Title        string
	Text         string
	TextOrigin   string
	SourceLang   string
	CanonicalURL *string
}

type modelNameProvider interface {
	ModelName() string
}

func modelNameFromProvider(provider Provider) *string {
	namedProvider, ok := provider.(modelNameProvider)
	if !ok {
		return nil
	}
	model := strings.TrimSpace(namedProvider.ModelName())
	if model == "" {
		return nil
	}
	return &model
}

func hashTranslationSourceText(text string) []byte {
	sum := sha256.Sum256([]byte(text))
	return sum[:]
}

func normalizeContentOrigin(origin string) string {
	switch strings.ToLower(strings.TrimSpace(origin)) {
	case ContentOriginReader:
		return ContentOriginReader
	default:
		return ContentOriginNormalized
	}
}

func normalizeCollection(raw string) string {
	return strings.TrimSpace(strings.ToLower(raw))
}
