package translation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"horse.fit/scoop/internal/db"
)

const (
	SourceTypeStoryTitle   = "story_title"
	SourceTypeStorySummary = "story_summary"
	SourceTypeArticleTitle = "article_title"
	SourceTypeArticleText  = "article_text"
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

	article, err := m.fetchArticleByUUID(ctx, articleUUID)
	if err != nil {
		return RunStats{}, err
	}

	tasks := make([]translationTask, 0, 2)
	if strings.TrimSpace(article.Title) != "" {
		tasks = append(tasks, translationTask{
			SourceType:   SourceTypeArticleTitle,
			SourceID:     article.ArticleID,
			SourceLang:   article.SourceLang,
			OriginalText: article.Title,
		})
	}
	if strings.TrimSpace(article.Text) != "" {
		tasks = append(tasks, translationTask{
			SourceType:   SourceTypeArticleText,
			SourceID:     article.ArticleID,
			SourceLang:   article.SourceLang,
			OriginalText: article.Text,
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
	t.translation_uuid::text,
	t.source_type,
	t.source_id,
	CASE
		WHEN t.source_type = 'story_title' THEN s.story_uuid::text
		WHEN t.source_type IN ('article_title', 'article_text') THEN a.article_uuid::text
		ELSE NULL
	END AS source_uuid,
	t.source_lang,
	t.target_lang,
	t.original_text,
	t.translated_text,
	t.provider_name,
	t.model_name,
	t.latency_ms,
	t.created_at
FROM news.translations t
LEFT JOIN news.stories s
	ON t.source_type = 'story_title'
	AND s.story_id = t.source_id
LEFT JOIN news.articles a
	ON t.source_type IN ('article_title', 'article_text')
	AND a.article_id = t.source_id
WHERE (t.source_type = 'story_title' AND t.source_id = $1)
   OR (
		t.source_type IN ('article_title', 'article_text')
		AND t.source_id IN (
			SELECT sa.article_id
			FROM news.story_articles sa
			WHERE sa.story_id = $1
		)
	)
ORDER BY t.target_lang, t.source_type, t.source_id, t.created_at DESC
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
	articles, err := m.fetchStoryArticles(ctx, story.StoryID)
	if err != nil {
		return RunStats{}, err
	}

	tasks := make([]translationTask, 0, 1+(2*len(articles)))
	if strings.TrimSpace(story.Title) != "" {
		tasks = append(tasks, translationTask{
			SourceType:   SourceTypeStoryTitle,
			SourceID:     story.StoryID,
			SourceLang:   story.SourceLang,
			OriginalText: story.Title,
		})
	}

	for _, article := range articles {
		if strings.TrimSpace(article.Title) != "" {
			tasks = append(tasks, translationTask{
				SourceType:   SourceTypeArticleTitle,
				SourceID:     article.ArticleID,
				SourceLang:   article.SourceLang,
				OriginalText: article.Title,
			})
		}
		if strings.TrimSpace(article.Text) != "" {
			tasks = append(tasks, translationTask{
				SourceType:   SourceTypeArticleText,
				SourceID:     article.ArticleID,
				SourceLang:   article.SourceLang,
				OriginalText: article.Text,
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

		cached, err := m.lookupCachedTranslation(ctx, task.SourceType, task.SourceID, targetLang)
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
			Text:       task.OriginalText,
			SourceLang: task.SourceLang,
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

		resolvedProviderName := strings.TrimSpace(resp.ProviderName)
		if resolvedProviderName == "" {
			resolvedProviderName = providerName
		}

		latencyMS := int(resp.LatencyMs)
		if latencyMS < 0 {
			latencyMS = 0
		}

		if err := m.upsertTranslation(ctx, upsertTranslationInput{
			SourceType:     task.SourceType,
			SourceID:       task.SourceID,
			SourceLang:     resolvedSourceLang,
			TargetLang:     resolvedTargetLang,
			OriginalText:   task.OriginalText,
			TranslatedText: translatedText,
			ProviderName:   resolvedProviderName,
			ModelName:      modelName,
			LatencyMS:      &latencyMS,
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
	a.normalized_language
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
		); err != nil {
			return nil, fmt.Errorf("scan story article row: %w", err)
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate story articles: %w", err)
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
	a.normalized_language
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
	)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return articleTranslationTarget{}, ErrArticleNotFound
		}
		return articleTranslationTarget{}, fmt.Errorf("query article: %w", err)
	}
	return row, nil
}

func (m *Manager) lookupCachedTranslation(
	ctx context.Context,
	sourceType string,
	sourceID int64,
	targetLang string,
) (*CachedTranslation, error) {
	const q = `
SELECT
	t.translation_uuid::text,
	t.source_type,
	t.source_id,
	t.source_lang,
	t.target_lang,
	t.original_text,
	t.translated_text,
	t.provider_name,
	t.model_name,
	t.latency_ms,
	t.created_at
FROM news.translations t
WHERE t.source_type = $1
  AND t.source_id = $2
  AND t.target_lang = $3
LIMIT 1
`

	var row CachedTranslation
	err := m.pool.QueryRow(ctx, q, sourceType, sourceID, targetLang).Scan(
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

type upsertTranslationInput struct {
	SourceType     string
	SourceID       int64
	SourceLang     string
	TargetLang     string
	OriginalText   string
	TranslatedText string
	ProviderName   string
	ModelName      *string
	LatencyMS      *int
}

func (m *Manager) upsertTranslation(ctx context.Context, row upsertTranslationInput) error {
	const q = `
INSERT INTO news.translations (
	source_type,
	source_id,
	source_lang,
	target_lang,
	original_text,
	translated_text,
	provider_name,
	model_name,
	latency_ms
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (source_type, source_id, target_lang)
DO UPDATE SET
	source_lang = EXCLUDED.source_lang,
	original_text = EXCLUDED.original_text,
	translated_text = EXCLUDED.translated_text,
	provider_name = EXCLUDED.provider_name,
	model_name = EXCLUDED.model_name,
	latency_ms = EXCLUDED.latency_ms,
	created_at = now()
`

	if _, err := m.pool.Exec(
		ctx,
		q,
		row.SourceType,
		row.SourceID,
		row.SourceLang,
		row.TargetLang,
		row.OriginalText,
		row.TranslatedText,
		row.ProviderName,
		row.ModelName,
		row.LatencyMS,
	); err != nil {
		return fmt.Errorf("upsert translation cache: %w", err)
	}
	return nil
}

type translationTask struct {
	SourceType   string
	SourceID     int64
	SourceLang   string
	OriginalText string
}

type storyTranslationTarget struct {
	StoryID    int64
	StoryUUID  string
	Title      string
	SourceLang string
}

type articleTranslationTarget struct {
	ArticleID   int64
	ArticleUUID string
	Title       string
	Text        string
	SourceLang  string
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

func normalizeCollection(raw string) string {
	return strings.TrimSpace(strings.ToLower(raw))
}
