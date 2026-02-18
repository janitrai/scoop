package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"

	"horse.fit/scoop/internal/db"
	"horse.fit/scoop/internal/globaltime"
)

const (
	defaultPageSize = 25
	maxPageSize     = 200
)

var errStoryNotFound = errors.New("story not found")

type Options struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

type Server struct {
	pool   *db.Pool
	logger zerolog.Logger
	opts   Options
}

type storyListFilter struct {
	Collection string
	Status     string
	Query      string
	From       *time.Time
	To         *time.Time
	Page       int
	PageSize   int
}

type storyRepresentative struct {
	ArticleUUID  string     `json:"article_uuid"`
	Source       string     `json:"source"`
	SourceItemID string     `json:"source_item_id"`
	PublishedAt  *time.Time `json:"published_at,omitempty"`
}

type storyListItem struct {
	StoryID        int64                `json:"story_id"`
	StoryUUID      string               `json:"story_uuid"`
	Collection     string               `json:"collection"`
	Title          string               `json:"title"`
	CanonicalURL   *string              `json:"canonical_url,omitempty"`
	Status         string               `json:"status"`
	FirstSeenAt    time.Time            `json:"first_seen_at"`
	LastSeenAt     time.Time            `json:"last_seen_at"`
	SourceCount    int                  `json:"source_count"`
	ArticleCount   int                  `json:"article_count"`
	Representative *storyRepresentative `json:"representative,omitempty"`
}

type StoryArticle struct {
	StoryArticleUUID     string         `json:"story_article_uuid"`
	ArticleUUID          string         `json:"article_uuid"`
	Source               string         `json:"source"`
	SourceItemID         string         `json:"source_item_id"`
	Collection           string         `json:"collection"`
	CanonicalURL         *string        `json:"canonical_url,omitempty"`
	PublishedAt          *time.Time     `json:"published_at,omitempty"`
	NormalizedTitle      string         `json:"normalized_title"`
	NormalizedText       string         `json:"normalized_text,omitempty"`
	SourceDomain         *string        `json:"source_domain,omitempty"`
	MatchedAt            time.Time      `json:"matched_at"`
	MatchType            string         `json:"match_type"`
	MatchScore           *float64       `json:"match_score,omitempty"`
	MatchDetails         map[string]any `json:"match_details,omitempty"`
	DedupDecision        *string        `json:"dedup_decision,omitempty"`
	DedupExactSignal     *string        `json:"dedup_exact_signal,omitempty"`
	DedupBestCosine      *float64       `json:"dedup_best_cosine,omitempty"`
	DedupTitleOverlap    *float64       `json:"dedup_title_overlap,omitempty"`
	DedupDateConsistency *float64       `json:"dedup_date_consistency,omitempty"`
	DedupCompositeScore  *float64       `json:"dedup_composite_score,omitempty"`
}

type storyDetail struct {
	Story   storyListItem  `json:"story"`
	Members []StoryArticle `json:"members"`
}

type collectionSummary struct {
	Collection      string     `json:"collection"`
	Articles        int64      `json:"articles"`
	Stories         int64      `json:"stories"`
	StoryItems      int64      `json:"story_items"`
	LastStorySeenAt *time.Time `json:"last_story_seen_at,omitempty"`
}

type statsResponse struct {
	RawArrivals       int64            `json:"raw_arrivals"`
	Articles          int64            `json:"articles"`
	Stories           int64            `json:"stories"`
	StoryArticles     int64            `json:"story_articles"`
	DedupEvents       int64            `json:"dedup_events"`
	RunningIngestRuns int64            `json:"running_ingest_runs"`
	LastFetchedAt     *time.Time       `json:"last_fetched_at,omitempty"`
	LastStoryUpdated  *time.Time       `json:"last_story_updated,omitempty"`
	DedupDecisions    map[string]int64 `json:"dedup_decisions"`
}

type storyDayBucket struct {
	Day        string `json:"day"`
	StoryCount int64  `json:"story_count"`
}

type updateStoryRequest struct {
	Title      *string `json:"title"`
	Status     *string `json:"status"`
	Collection *string `json:"collection"`
	URL        *string `json:"url"`
}

type updateArticleRequest struct {
	Title      *string `json:"title"`
	Source     *string `json:"source"`
	Collection *string `json:"collection"`
	URL        *string `json:"url"`
}

type updatedStory struct {
	StoryUUID    string     `json:"story_uuid"`
	Title        string     `json:"title"`
	Status       string     `json:"status"`
	Collection   string     `json:"collection"`
	CanonicalURL *string    `json:"canonical_url,omitempty"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

type updatedArticle struct {
	ArticleUUID  string     `json:"article_uuid"`
	Title        string     `json:"title"`
	Source       string     `json:"source"`
	Collection   string     `json:"collection"`
	CanonicalURL *string    `json:"canonical_url,omitempty"`
	SourceDomain *string    `json:"source_domain,omitempty"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

func NewServer(pool *db.Pool, logger zerolog.Logger, opts Options) *Server {
	host := strings.TrimSpace(opts.Host)
	if host == "" {
		host = "0.0.0.0"
	}
	port := opts.Port
	if port <= 0 {
		port = 8090
	}
	readTimeout := opts.ReadTimeout
	if readTimeout <= 0 {
		readTimeout = 10 * time.Second
	}
	writeTimeout := opts.WriteTimeout
	if writeTimeout <= 0 {
		writeTimeout = 30 * time.Second
	}
	shutdownTimeout := opts.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = 10 * time.Second
	}

	return &Server{
		pool:   pool,
		logger: logger,
		opts: Options{
			Host:            host,
			Port:            port,
			ReadTimeout:     readTimeout,
			WriteTimeout:    writeTimeout,
			ShutdownTimeout: shutdownTimeout,
		},
	}
}

func (s *Server) Start(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("server is not initialized")
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = s.httpErrorHandler

	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodOptions, http.MethodPost, http.MethodPatch, http.MethodDelete},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
		MaxAge:       3600,
	}))
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:    true,
		LogURI:       true,
		LogMethod:    true,
		LogLatency:   true,
		LogRemoteIP:  true,
		LogRequestID: true,
		LogError:     true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			if v.Error != nil {
				s.logger.Error().
					Err(v.Error).
					Str("method", v.Method).
					Str("uri", v.URI).
					Int("status", v.Status).
					Dur("latency", v.Latency).
					Str("remote_ip", v.RemoteIP).
					Str("request_id", v.RequestID).
					Msg("http request failed")
				return nil
			}

			s.logger.Info().
				Str("method", v.Method).
				Str("uri", v.URI).
				Int("status", v.Status).
				Dur("latency", v.Latency).
				Str("remote_ip", v.RemoteIP).
				Str("request_id", v.RequestID).
				Msg("http request")
			return nil
		},
	}))
	e.GET("/", func(c echo.Context) error {
		return success(c, map[string]any{
			"service": "scoop-api",
			"status":  "ok",
			"time":    globaltime.UTC(),
		})
	})

	api := e.Group("/api/v1")
	api.GET("/health", s.handleHealth)
	api.GET("/stats", s.handleStats)
	api.GET("/collections", s.handleCollections)
	api.GET("/story-days", s.handleStoryDays)
	api.GET("/stories", s.handleStories)
	api.GET("/stories/:story_uuid", s.handleStoryDetail)
	api.DELETE("/stories/:story_uuid", s.handleDeleteStory)
	api.PATCH("/stories/:story_uuid", s.handleUpdateStory)
	api.POST("/stories/:story_uuid/restore", s.handleRestoreStory)
	api.DELETE("/articles/:article_uuid", s.handleDeleteArticle)
	api.PATCH("/articles/:article_uuid", s.handleUpdateArticle)
	api.POST("/articles/:article_uuid/restore", s.handleRestoreArticle)
	api.GET("/articles/:story_article_uuid/preview", s.handleStoryArticlePreview)

	addr := fmt.Sprintf("%s:%d", s.opts.Host, s.opts.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      e,
		ReadTimeout:  s.opts.ReadTimeout,
		WriteTimeout: s.opts.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.opts.ShutdownTimeout)
		defer cancel()
		if shutdownErr := e.Shutdown(shutdownCtx); shutdownErr != nil {
			s.logger.Error().Err(shutdownErr).Msg("server shutdown failed")
		}
	}()

	s.logger.Info().Str("addr", addr).Msg("scoop web server started")

	if err := e.StartServer(httpServer); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("start server: %w", err)
	}
	s.logger.Info().Msg("scoop web server stopped")
	return nil
}

func (s *Server) httpErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	status := http.StatusInternalServerError
	message := "Internal server error"
	if he, ok := err.(*echo.HTTPError); ok {
		status = he.Code
		switch v := he.Message.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				message = v
			}
		default:
			if text := strings.TrimSpace(http.StatusText(status)); text != "" {
				message = text
			}
		}
	} else if err != nil {
		message = err.Error()
	}

	isAPI := strings.HasPrefix(c.Request().URL.Path, "/api/")
	if isAPI {
		if status >= 500 {
			_ = internalError(c, "Internal server error")
			return
		}
		_ = fail(c, status, message, nil)
		return
	}

	_ = c.String(status, message)
}

func (s *Server) handleHealth(c echo.Context) error {
	return success(c, map[string]any{
		"service": "scoop",
		"time":    globaltime.UTC(),
	})
}

func (s *Server) handleStats(c echo.Context) error {
	stats, err := s.queryStats(c.Request().Context())
	if err != nil {
		s.logger.Error().Err(err).Msg("query stats failed")
		return internalError(c, "Failed to load stats")
	}
	return success(c, stats)
}

func (s *Server) handleCollections(c echo.Context) error {
	rows, err := s.queryCollections(c.Request().Context())
	if err != nil {
		s.logger.Error().Err(err).Msg("query collections failed")
		return internalError(c, "Failed to load collections")
	}
	return success(c, map[string]any{
		"items": rows,
	})
}

func (s *Server) handleStoryDays(c echo.Context) error {
	limit, err := parsePositiveInt(c.QueryParam("limit"), 30, 1, 180)
	if err != nil {
		return failValidation(c, map[string]string{"limit": err.Error()})
	}

	collection := normalizeCollection(c.QueryParam("collection"))
	items, err := s.queryStoryDays(c.Request().Context(), collection, limit)
	if err != nil {
		s.logger.Error().Err(err).Str("collection", collection).Msg("query story day buckets failed")
		return internalError(c, "Failed to load story day buckets")
	}

	return success(c, map[string]any{
		"items":      items,
		"collection": collection,
		"limit":      limit,
	})
}

func (s *Server) handleStories(c echo.Context) error {
	page, err := parsePositiveInt(c.QueryParam("page"), 1, 1, 1_000_000)
	if err != nil {
		return failValidation(c, map[string]string{"page": err.Error()})
	}

	pageSize, err := parsePositiveInt(c.QueryParam("page_size"), defaultPageSize, 1, maxPageSize)
	if err != nil {
		return failValidation(c, map[string]string{"page_size": err.Error()})
	}

	from, err := parseTimeFilter(c.QueryParam("from"), false)
	if err != nil {
		return failValidation(c, map[string]string{"from": "must be RFC3339 or YYYY-MM-DD"})
	}
	to, err := parseTimeFilter(c.QueryParam("to"), true)
	if err != nil {
		return failValidation(c, map[string]string{"to": "must be RFC3339 or YYYY-MM-DD"})
	}
	if from != nil && to != nil && from.After(*to) {
		return failValidation(c, map[string]string{"time_range": "from must be <= to"})
	}

	filter := storyListFilter{
		Collection: normalizeCollection(c.QueryParam("collection")),
		Status:     strings.TrimSpace(strings.ToLower(c.QueryParam("status"))),
		Query:      strings.TrimSpace(c.QueryParam("q")),
		From:       from,
		To:         to,
		Page:       page,
		PageSize:   pageSize,
	}

	total, rows, err := s.queryStoryList(c.Request().Context(), filter)
	if err != nil {
		s.logger.Error().Err(err).Msg("query stories failed")
		return internalError(c, "Failed to load stories")
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return success(c, map[string]any{
		"items": rows,
		"pagination": map[string]any{
			"page":        page,
			"page_size":   pageSize,
			"total_items": total,
			"total_pages": totalPages,
		},
		"filters": map[string]any{
			"collection": filter.Collection,
			"status":     filter.Status,
			"q":          filter.Query,
			"from":       filter.From,
			"to":         filter.To,
		},
	})
}

func (s *Server) handleStoryDetail(c echo.Context) error {
	storyUUID := strings.TrimSpace(c.Param("story_uuid"))
	if storyUUID == "" {
		return failValidation(c, map[string]string{"story_uuid": "is required"})
	}

	detail, err := s.queryStoryDetail(c.Request().Context(), storyUUID)
	if err != nil {
		if errors.Is(err, errStoryNotFound) {
			return failNotFound(c, "Story not found")
		}
		s.logger.Error().Err(err).Str("story_uuid", storyUUID).Msg("query story detail failed")
		return internalError(c, "Failed to load story detail")
	}

	return success(c, detail)
}

func (s *Server) handleDeleteStory(c echo.Context) error {
	storyUUID := strings.TrimSpace(c.Param("story_uuid"))
	if storyUUID == "" {
		return failValidation(c, map[string]string{"story_uuid": "is required"})
	}

	affected, err := s.pool.SoftDeleteStory(c.Request().Context(), storyUUID, globaltime.UTC())
	if err != nil {
		if msg := mutationValidationMessage(err); msg != "" {
			return failValidation(c, map[string]string{"story_uuid": msg})
		}
		s.logger.Error().Err(err).Str("story_uuid", storyUUID).Msg("soft delete story failed")
		return internalError(c, "Failed to soft delete story")
	}
	if affected == 0 {
		return failNotFound(c, "Story not found")
	}

	return success(c, map[string]any{
		"story_uuid": storyUUID,
		"affected":   affected,
	})
}

func (s *Server) handleRestoreStory(c echo.Context) error {
	storyUUID := strings.TrimSpace(c.Param("story_uuid"))
	if storyUUID == "" {
		return failValidation(c, map[string]string{"story_uuid": "is required"})
	}

	affected, err := s.pool.RestoreStory(c.Request().Context(), storyUUID, globaltime.UTC())
	if err != nil {
		if msg := mutationValidationMessage(err); msg != "" {
			return failValidation(c, map[string]string{"story_uuid": msg})
		}
		s.logger.Error().Err(err).Str("story_uuid", storyUUID).Msg("restore story failed")
		return internalError(c, "Failed to restore story")
	}
	if affected == 0 {
		return failNotFound(c, "Story not found")
	}

	return success(c, map[string]any{
		"story_uuid": storyUUID,
		"affected":   affected,
	})
}

func (s *Server) handleUpdateStory(c echo.Context) error {
	storyUUID := strings.TrimSpace(c.Param("story_uuid"))
	if storyUUID == "" {
		return failValidation(c, map[string]string{"story_uuid": "is required"})
	}

	var req updateStoryRequest
	if err := decodeJSONBody(c, &req); err != nil {
		return failValidation(c, map[string]string{"body": err.Error()})
	}

	opts := db.UpdateStoryOptions{
		Title:      req.Title,
		Status:     req.Status,
		Collection: req.Collection,
		URL:        req.URL,
	}
	if opts.Title == nil && opts.Status == nil && opts.Collection == nil && opts.URL == nil {
		return failValidation(c, map[string]string{"body": "at least one update field is required"})
	}

	if err := s.pool.UpdateStory(c.Request().Context(), storyUUID, opts, globaltime.UTC()); err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return failNotFound(c, "Story not found")
		}
		if msg := mutationValidationMessage(err); msg != "" {
			return failValidation(c, map[string]string{"body": msg})
		}
		s.logger.Error().Err(err).Str("story_uuid", storyUUID).Msg("update story failed")
		return internalError(c, "Failed to update story")
	}

	row, err := s.queryUpdatedStory(c.Request().Context(), storyUUID)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return failNotFound(c, "Story not found")
		}
		s.logger.Error().Err(err).Str("story_uuid", storyUUID).Msg("query updated story failed")
		return internalError(c, "Failed to load updated story")
	}
	return success(c, map[string]any{"story": row})
}

func (s *Server) handleDeleteArticle(c echo.Context) error {
	articleUUID := strings.TrimSpace(c.Param("article_uuid"))
	if articleUUID == "" {
		return failValidation(c, map[string]string{"article_uuid": "is required"})
	}

	affected, err := s.pool.SoftDeleteArticle(c.Request().Context(), articleUUID, globaltime.UTC())
	if err != nil {
		if msg := mutationValidationMessage(err); msg != "" {
			return failValidation(c, map[string]string{"article_uuid": msg})
		}
		s.logger.Error().Err(err).Str("article_uuid", articleUUID).Msg("soft delete article failed")
		return internalError(c, "Failed to soft delete article")
	}
	if affected == 0 {
		return failNotFound(c, "Article not found")
	}

	return success(c, map[string]any{
		"article_uuid": articleUUID,
		"affected":     affected,
	})
}

func (s *Server) handleRestoreArticle(c echo.Context) error {
	articleUUID := strings.TrimSpace(c.Param("article_uuid"))
	if articleUUID == "" {
		return failValidation(c, map[string]string{"article_uuid": "is required"})
	}

	affected, err := s.pool.RestoreArticle(c.Request().Context(), articleUUID, globaltime.UTC())
	if err != nil {
		if msg := mutationValidationMessage(err); msg != "" {
			return failValidation(c, map[string]string{"article_uuid": msg})
		}
		s.logger.Error().Err(err).Str("article_uuid", articleUUID).Msg("restore article failed")
		return internalError(c, "Failed to restore article")
	}
	if affected == 0 {
		return failNotFound(c, "Article not found")
	}

	return success(c, map[string]any{
		"article_uuid": articleUUID,
		"affected":     affected,
	})
}

func (s *Server) handleUpdateArticle(c echo.Context) error {
	articleUUID := strings.TrimSpace(c.Param("article_uuid"))
	if articleUUID == "" {
		return failValidation(c, map[string]string{"article_uuid": "is required"})
	}

	var req updateArticleRequest
	if err := decodeJSONBody(c, &req); err != nil {
		return failValidation(c, map[string]string{"body": err.Error()})
	}

	opts := db.UpdateArticleOptions{
		Title:      req.Title,
		Source:     req.Source,
		Collection: req.Collection,
		URL:        req.URL,
	}
	if opts.Title == nil && opts.Source == nil && opts.Collection == nil && opts.URL == nil {
		return failValidation(c, map[string]string{"body": "at least one update field is required"})
	}

	if err := s.pool.UpdateArticle(c.Request().Context(), articleUUID, opts, globaltime.UTC()); err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return failNotFound(c, "Article not found")
		}
		if msg := mutationValidationMessage(err); msg != "" {
			return failValidation(c, map[string]string{"body": msg})
		}
		s.logger.Error().Err(err).Str("article_uuid", articleUUID).Msg("update article failed")
		return internalError(c, "Failed to update article")
	}

	row, err := s.queryUpdatedArticle(c.Request().Context(), articleUUID)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return failNotFound(c, "Article not found")
		}
		s.logger.Error().Err(err).Str("article_uuid", articleUUID).Msg("query updated article failed")
		return internalError(c, "Failed to load updated article")
	}
	return success(c, map[string]any{"article": row})
}

func (s *Server) queryStoryList(ctx context.Context, filter storyListFilter) (int64, []storyListItem, error) {
	search := ""
	if strings.TrimSpace(filter.Query) != "" {
		search = "%" + strings.TrimSpace(filter.Query) + "%"
	}

	const countQuery = `
SELECT COUNT(*)
FROM news.stories s
WHERE s.deleted_at IS NULL
  AND ($1 = '' OR s.collection = $1)
  AND ($2 = '' OR s.status = $2)
  AND ($3 = '' OR s.canonical_title ILIKE $3 OR COALESCE(s.canonical_url, '') ILIKE $3)
  AND ($4::timestamptz IS NULL OR s.last_seen_at >= $4)
  AND ($5::timestamptz IS NULL OR s.last_seen_at <= $5)
`

	var total int64
	if err := s.pool.QueryRow(ctx, countQuery, filter.Collection, filter.Status, search, filter.From, filter.To).Scan(&total); err != nil {
		return 0, nil, fmt.Errorf("count stories: %w", err)
	}

	offset := (filter.Page - 1) * filter.PageSize

	const rowsQuery = `
SELECT
	s.story_id,
	s.story_uuid::text,
	s.collection,
	s.canonical_title,
	s.canonical_url,
	s.status,
	s.first_seen_at,
	s.last_seen_at,
	(SELECT COUNT(DISTINCT a.source)
	 FROM news.story_articles sa
	 JOIN news.articles a
		ON a.article_id = sa.article_id
		AND a.deleted_at IS NULL
	 WHERE sa.story_id = s.story_id) AS source_count,
	(SELECT COUNT(*)
	 FROM news.story_articles sa
	 JOIN news.articles a
		ON a.article_id = sa.article_id
		AND a.deleted_at IS NULL
	 WHERE sa.story_id = s.story_id) AS article_count,
	rd.article_uuid::text,
	rd.source,
	rd.source_item_id,
	rd.published_at
FROM news.stories s
LEFT JOIN news.articles rd
	ON rd.article_id = s.representative_article_id
	AND rd.deleted_at IS NULL
WHERE s.deleted_at IS NULL
  AND ($1 = '' OR s.collection = $1)
  AND ($2 = '' OR s.status = $2)
  AND ($3 = '' OR s.canonical_title ILIKE $3 OR COALESCE(s.canonical_url, '') ILIKE $3)
  AND ($4::timestamptz IS NULL OR s.last_seen_at >= $4)
  AND ($5::timestamptz IS NULL OR s.last_seen_at <= $5)
ORDER BY s.last_seen_at DESC, s.story_id DESC
LIMIT $6
OFFSET $7
`

	rows, err := s.pool.Query(ctx, rowsQuery, filter.Collection, filter.Status, search, filter.From, filter.To, filter.PageSize, offset)
	if err != nil {
		return 0, nil, fmt.Errorf("query stories: %w", err)
	}
	defer rows.Close()

	items := make([]storyListItem, 0, filter.PageSize)
	for rows.Next() {
		var (
			row             storyListItem
			repArticleUUID  *string
			repSource       *string
			repSourceItemID *string
			repPublishedAt  *time.Time
		)
		if err := rows.Scan(
			&row.StoryID,
			&row.StoryUUID,
			&row.Collection,
			&row.Title,
			&row.CanonicalURL,
			&row.Status,
			&row.FirstSeenAt,
			&row.LastSeenAt,
			&row.SourceCount,
			&row.ArticleCount,
			&repArticleUUID,
			&repSource,
			&repSourceItemID,
			&repPublishedAt,
		); err != nil {
			return 0, nil, fmt.Errorf("scan story row: %w", err)
		}

		if repArticleUUID != nil && repSource != nil && repSourceItemID != nil {
			row.Representative = &storyRepresentative{
				ArticleUUID:  *repArticleUUID,
				Source:       *repSource,
				SourceItemID: *repSourceItemID,
				PublishedAt:  repPublishedAt,
			}
		}
		items = append(items, row)
	}

	if err := rows.Err(); err != nil {
		return 0, nil, fmt.Errorf("iterate story rows: %w", err)
	}

	return total, items, nil
}

func (s *Server) queryUpdatedStory(ctx context.Context, storyUUID string) (*updatedStory, error) {
	const q = `
SELECT
	s.story_uuid::text,
	s.canonical_title,
	s.status,
	s.collection,
	s.canonical_url,
	s.updated_at,
	s.deleted_at
FROM news.stories s
WHERE s.story_uuid = $1::uuid
LIMIT 1
`
	var row updatedStory
	if err := s.pool.QueryRow(ctx, q, storyUUID).Scan(
		&row.StoryUUID,
		&row.Title,
		&row.Status,
		&row.Collection,
		&row.CanonicalURL,
		&row.UpdatedAt,
		&row.DeletedAt,
	); err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return nil, db.ErrNoRows
		}
		return nil, fmt.Errorf("query updated story: %w", err)
	}
	return &row, nil
}

func (s *Server) queryUpdatedArticle(ctx context.Context, articleUUID string) (*updatedArticle, error) {
	const q = `
SELECT
	a.article_uuid::text,
	a.normalized_title,
	a.source,
	a.collection,
	a.canonical_url,
	a.source_domain,
	a.updated_at,
	a.deleted_at
FROM news.articles a
WHERE a.article_uuid = $1::uuid
LIMIT 1
`
	var row updatedArticle
	if err := s.pool.QueryRow(ctx, q, articleUUID).Scan(
		&row.ArticleUUID,
		&row.Title,
		&row.Source,
		&row.Collection,
		&row.CanonicalURL,
		&row.SourceDomain,
		&row.UpdatedAt,
		&row.DeletedAt,
	); err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return nil, db.ErrNoRows
		}
		return nil, fmt.Errorf("query updated article: %w", err)
	}
	return &row, nil
}

func (s *Server) queryStoryDetail(ctx context.Context, storyUUID string) (*storyDetail, error) {
	const storyQuery = `
SELECT
	s.story_id,
	s.story_uuid::text,
	s.collection,
	s.canonical_title,
	s.canonical_url,
	s.status,
	s.first_seen_at,
	s.last_seen_at,
	(SELECT COUNT(DISTINCT a.source)
	 FROM news.story_articles sa
	 JOIN news.articles a
		ON a.article_id = sa.article_id
		AND a.deleted_at IS NULL
	 WHERE sa.story_id = s.story_id) AS source_count,
	(SELECT COUNT(*)
	 FROM news.story_articles sa
	 JOIN news.articles a
		ON a.article_id = sa.article_id
		AND a.deleted_at IS NULL
	 WHERE sa.story_id = s.story_id) AS article_count,
	rd.article_uuid::text,
	rd.source,
	rd.source_item_id,
	rd.published_at
FROM news.stories s
LEFT JOIN news.articles rd
	ON rd.article_id = s.representative_article_id
	AND rd.deleted_at IS NULL
WHERE s.story_uuid = $1::uuid
  AND s.deleted_at IS NULL
`

	var (
		story           storyListItem
		repArticleUUID  *string
		repSource       *string
		repSourceItemID *string
		repPublishedAt  *time.Time
	)
	if err := s.pool.QueryRow(ctx, storyQuery, storyUUID).Scan(
		&story.StoryID,
		&story.StoryUUID,
		&story.Collection,
		&story.Title,
		&story.CanonicalURL,
		&story.Status,
		&story.FirstSeenAt,
		&story.LastSeenAt,
		&story.SourceCount,
		&story.ArticleCount,
		&repArticleUUID,
		&repSource,
		&repSourceItemID,
		&repPublishedAt,
	); err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return nil, errStoryNotFound
		}
		return nil, fmt.Errorf("query story: %w", err)
	}

	if repArticleUUID != nil && repSource != nil && repSourceItemID != nil {
		story.Representative = &storyRepresentative{
			ArticleUUID:  *repArticleUUID,
			Source:       *repSource,
			SourceItemID: *repSourceItemID,
			PublishedAt:  repPublishedAt,
		}
	}

	const membersQuery = `
SELECT
	sm.story_article_uuid::text,
	d.article_uuid::text,
	d.source,
	d.source_item_id,
	d.collection,
	d.canonical_url,
	d.published_at,
	d.normalized_title,
	d.normalized_text,
	d.source_domain,
	sm.matched_at,
	sm.match_type::text,
	sm.match_score,
	sm.match_details,
	de.decision::text,
	de.exact_signal,
	de.best_cosine,
	de.title_overlap,
	de.entity_date_consistency,
	de.composite_score
FROM news.story_articles sm
JOIN news.articles d
	ON d.article_id = sm.article_id
	AND d.deleted_at IS NULL
LEFT JOIN news.dedup_events de
	ON de.article_id = d.article_id
WHERE sm.story_id = $1
ORDER BY sm.matched_at DESC
`

	rows, err := s.pool.Query(ctx, membersQuery, story.StoryID)
	if err != nil {
		return nil, fmt.Errorf("query story articles: %w", err)
	}
	defer rows.Close()

	members := make([]StoryArticle, 0, story.ArticleCount)
	for rows.Next() {
		var (
			member          StoryArticle
			matchDetailsRaw []byte
		)
		if err := rows.Scan(
			&member.StoryArticleUUID,
			&member.ArticleUUID,
			&member.Source,
			&member.SourceItemID,
			&member.Collection,
			&member.CanonicalURL,
			&member.PublishedAt,
			&member.NormalizedTitle,
			&member.NormalizedText,
			&member.SourceDomain,
			&member.MatchedAt,
			&member.MatchType,
			&member.MatchScore,
			&matchDetailsRaw,
			&member.DedupDecision,
			&member.DedupExactSignal,
			&member.DedupBestCosine,
			&member.DedupTitleOverlap,
			&member.DedupDateConsistency,
			&member.DedupCompositeScore,
		); err != nil {
			return nil, fmt.Errorf("scan story article: %w", err)
		}

		if len(matchDetailsRaw) > 0 && string(matchDetailsRaw) != "null" {
			var details map[string]any
			if err := json.Unmarshal(matchDetailsRaw, &details); err == nil {
				member.MatchDetails = details
			}
		}

		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate story articles: %w", err)
	}

	return &storyDetail{
		Story:   story,
		Members: members,
	}, nil
}

func (s *Server) queryCollections(ctx context.Context) ([]collectionSummary, error) {
	const q = `
WITH article_counts AS (
	SELECT collection, COUNT(*)::BIGINT AS articles
	FROM news.articles
	WHERE deleted_at IS NULL
	GROUP BY collection
),
story_counts AS (
	SELECT
		s.collection,
		COUNT(*)::BIGINT AS stories,
		COALESCE(SUM(
			(SELECT COUNT(*)
			 FROM news.story_articles sa
			 JOIN news.articles a
				ON a.article_id = sa.article_id
				AND a.deleted_at IS NULL
			 WHERE sa.story_id = s.story_id)
		), 0)::BIGINT AS story_items,
		MAX(s.last_seen_at) AS last_story_seen_at
	FROM news.stories s
	WHERE s.deleted_at IS NULL
	GROUP BY s.collection
)
SELECT
	COALESCE(d.collection, s.collection) AS collection,
	COALESCE(d.articles, 0) AS articles,
	COALESCE(s.stories, 0) AS stories,
	COALESCE(s.story_items, 0) AS story_items,
	s.last_story_seen_at
FROM article_counts d
FULL OUTER JOIN story_counts s
	ON s.collection = d.collection
ORDER BY 1
`

	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query collection summary: %w", err)
	}
	defer rows.Close()

	items := make([]collectionSummary, 0, 8)
	for rows.Next() {
		var row collectionSummary
		if err := rows.Scan(&row.Collection, &row.Articles, &row.Stories, &row.StoryItems, &row.LastStorySeenAt); err != nil {
			return nil, fmt.Errorf("scan collection summary: %w", err)
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate collection summary: %w", err)
	}
	return items, nil
}

func (s *Server) queryStoryDays(ctx context.Context, collection string, limit int) ([]storyDayBucket, error) {
	const q = `
SELECT
	TO_CHAR((s.last_seen_at AT TIME ZONE 'Europe/Berlin')::date, 'YYYY-MM-DD') AS day_bucket,
	COUNT(*)::BIGINT AS story_count
FROM news.stories s
WHERE s.deleted_at IS NULL
  AND ($1 = '' OR s.collection = $1)
GROUP BY day_bucket
ORDER BY day_bucket DESC
LIMIT $2
`
	rows, err := s.pool.Query(ctx, q, collection, limit)
	if err != nil {
		return nil, fmt.Errorf("query story day buckets: %w", err)
	}
	defer rows.Close()

	items := make([]storyDayBucket, 0, limit)
	for rows.Next() {
		var row storyDayBucket
		if err := rows.Scan(&row.Day, &row.StoryCount); err != nil {
			return nil, fmt.Errorf("scan story day bucket: %w", err)
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate story day buckets: %w", err)
	}
	return items, nil
}

func (s *Server) queryStats(ctx context.Context) (*statsResponse, error) {
	const q = `
SELECT
	(SELECT COUNT(*) FROM news.raw_arrivals WHERE deleted_at IS NULL) AS raw_arrivals,
	(SELECT COUNT(*) FROM news.articles WHERE deleted_at IS NULL) AS articles,
	(SELECT COUNT(*) FROM news.stories WHERE deleted_at IS NULL) AS stories,
	(SELECT COUNT(*) FROM news.story_articles) AS story_articles,
	(SELECT COUNT(*) FROM news.dedup_events) AS dedup_events,
	(SELECT COUNT(*) FROM news.ingest_runs WHERE status = 'running') AS running_ingest_runs,
	(SELECT MAX(fetched_at) FROM news.raw_arrivals WHERE deleted_at IS NULL) AS last_fetched_at,
	(SELECT MAX(updated_at) FROM news.stories WHERE deleted_at IS NULL) AS last_story_updated
`

	var stats statsResponse
	if err := s.pool.QueryRow(ctx, q).Scan(
		&stats.RawArrivals,
		&stats.Articles,
		&stats.Stories,
		&stats.StoryArticles,
		&stats.DedupEvents,
		&stats.RunningIngestRuns,
		&stats.LastFetchedAt,
		&stats.LastStoryUpdated,
	); err != nil {
		return nil, fmt.Errorf("query stats: %w", err)
	}

	const decisionQuery = `
SELECT decision::text, COUNT(*)::BIGINT
FROM news.dedup_events
GROUP BY decision
ORDER BY decision
`
	rows, err := s.pool.Query(ctx, decisionQuery)
	if err != nil {
		return nil, fmt.Errorf("query dedup decisions: %w", err)
	}
	defer rows.Close()

	stats.DedupDecisions = map[string]int64{}
	for rows.Next() {
		var decision string
		var count int64
		if err := rows.Scan(&decision, &count); err != nil {
			return nil, fmt.Errorf("scan dedup decision: %w", err)
		}
		stats.DedupDecisions[decision] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dedup decisions: %w", err)
	}

	return &stats, nil
}

func normalizeCollection(raw string) string {
	return strings.TrimSpace(strings.ToLower(raw))
}

func decodeJSONBody(c echo.Context, dst any) error {
	if c == nil || c.Request() == nil || c.Request().Body == nil {
		return fmt.Errorf("request body is required")
	}

	decoder := json.NewDecoder(c.Request().Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("request body is required")
		}
		return fmt.Errorf("must be valid JSON")
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("must contain only one JSON object")
	}

	return nil
}

func mutationValidationMessage(err error) string {
	if err == nil {
		return ""
	}

	text := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(text, "invalid input syntax for type uuid"):
		return "must be a valid UUID"
	case strings.Contains(text, "uuid is required"):
		return "is required"
	case strings.Contains(text, "at least one update field is required"):
		return "at least one update field is required"
	case strings.Contains(text, "must not be empty"):
		return err.Error()
	case strings.Contains(text, "fully-qualified url"):
		return "url must be a fully-qualified URL"
	default:
		return ""
	}
}

func parsePositiveInt(raw string, defaultValue, minValue, maxValue int) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("must be an integer")
	}
	if value < minValue || value > maxValue {
		return 0, fmt.Errorf("must be between %d and %d", minValue, maxValue)
	}
	return value, nil
}

func parseTimeFilter(raw string, endOfDay bool) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	if ts, err := time.Parse(time.RFC3339, trimmed); err == nil {
		utc := ts.UTC()
		return &utc, nil
	}

	if day, err := time.Parse("2006-01-02", trimmed); err == nil {
		utc := day.UTC()
		if endOfDay {
			utc = utc.Add((24 * time.Hour) - time.Nanosecond)
		}
		return &utc, nil
	}

	return nil, fmt.Errorf("invalid time format")
}
