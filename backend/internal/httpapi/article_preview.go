package httpapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/labstack/echo/v4"

	"horse.fit/scoop/internal/db"
)

const (
	defaultArticlePreviewMaxChars = 1000
	minArticlePreviewMaxChars     = 200
	maxArticlePreviewMaxChars     = 4000
	readerFetchTimeout            = 12 * time.Second
	readerBodyByteLimit           = 2 * 1024 * 1024
)

var errStoryArticleNotFound = errors.New("story article not found")

type storyArticlePreview struct {
	StoryArticleUUID string  `json:"story_article_uuid"`
	PreviewText      string  `json:"preview_text"`
	Source           string  `json:"source"`
	CharCount        int     `json:"char_count"`
	Truncated        bool    `json:"truncated"`
	PreviewError     *string `json:"preview_error,omitempty"`
}

func (s *Server) handleStoryArticlePreview(c echo.Context) error {
	storyArticleUUID := strings.TrimSpace(c.Param("story_article_uuid"))
	if storyArticleUUID == "" {
		return failValidation(c, map[string]string{"story_article_uuid": "is required"})
	}

	maxChars, err := parsePositiveInt(
		c.QueryParam("max_chars"),
		defaultArticlePreviewMaxChars,
		minArticlePreviewMaxChars,
		maxArticlePreviewMaxChars,
	)
	if err != nil {
		return failValidation(c, map[string]string{"max_chars": err.Error()})
	}

	preview, err := s.queryStoryArticlePreview(c.Request().Context(), storyArticleUUID, maxChars)
	if err != nil {
		if errors.Is(err, errStoryArticleNotFound) {
			return failNotFound(c, "Story article not found")
		}
		s.logger.Error().Err(err).Str("story_article_uuid", storyArticleUUID).Msg("query story article preview failed")
		return internalError(c, "Failed to load story article preview")
	}

	return success(c, preview)
}

func (s *Server) queryStoryArticlePreview(ctx context.Context, storyArticleUUID string, maxChars int) (*storyArticlePreview, error) {
	const q = `
SELECT
	sm.story_article_uuid::text,
	d.normalized_text,
	d.canonical_url,
	d.normalized_title
FROM news.story_articles sm
JOIN news.articles d
	ON d.article_id = sm.article_id
WHERE sm.story_article_uuid = $1::uuid
LIMIT 1
`

	var (
		rowStoryArticleUUID string
		rowNormalizedText   *string
		rowCanonicalURL     *string
		rowTitle            string
	)

	if err := s.pool.QueryRow(ctx, q, storyArticleUUID).Scan(
		&rowStoryArticleUUID,
		&rowNormalizedText,
		&rowCanonicalURL,
		&rowTitle,
	); err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return nil, errStoryArticleNotFound
		}
		return nil, fmt.Errorf("query story article preview row: %w", err)
	}

	normalizedText := ""
	if rowNormalizedText != nil {
		normalizedText = strings.TrimSpace(*rowNormalizedText)
	}

	previewRaw, source, previewErr := buildArticlePreviewText(ctx, rowCanonicalURL, rowTitle, normalizedText)
	previewText, truncated := truncatePreviewText(previewRaw, maxChars)

	resp := &storyArticlePreview{
		StoryArticleUUID: rowStoryArticleUUID,
		PreviewText:      previewText,
		Source:           source,
		CharCount:        utf8.RuneCountInString(previewText),
		Truncated:        truncated,
	}
	if previewErr != nil {
		msg := previewErr.Error()
		resp.PreviewError = &msg
		s.logger.Warn().
			Err(previewErr).
			Str("story_article_uuid", storyArticleUUID).
			Str("source", source).
			Msg("reader preview fallback used")
	}

	return resp, nil
}

func buildArticlePreviewText(
	ctx context.Context,
	canonicalURL *string,
	normalizedTitle string,
	normalizedText string,
) (string, string, error) {
	url := strings.TrimSpace(derefString(canonicalURL))
	if url != "" {
		readerText, err := fetchReaderPreviewText(ctx, url, normalizedTitle)
		if err == nil && strings.TrimSpace(readerText) != "" {
			return readerText, "reader", nil
		}
		if normalizedText != "" {
			return normalizedText, "normalized_text", err
		}
		return "", "none", err
	}

	if normalizedText != "" {
		return normalizedText, "normalized_text", nil
	}

	return "", "none", nil
}

func fetchReaderPreviewText(ctx context.Context, canonicalURL string, title string) (string, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, readerFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, canonicalURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("User-Agent", "SCOOP-ReaderPreview/1.0 (+https://github.com/janitrai/scoop)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.8")

	client := &http.Client{
		Timeout: readerFetchTimeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, readerBodyByteLimit))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "text/plain") {
		return cleanPreviewText(string(body)), nil
	}

	pageURL, err := url.Parse(canonicalURL)
	if err != nil {
		return "", fmt.Errorf("parse page url: %w", err)
	}

	article, err := readability.FromReader(bytes.NewReader(body), pageURL)
	if err != nil {
		return "", fmt.Errorf("readability parse: %w", err)
	}

	var renderedText bytes.Buffer
	if err := article.RenderText(&renderedText); err != nil {
		return "", fmt.Errorf("render readability text: %w", err)
	}

	text := cleanPreviewText(renderedText.String())
	if text == "" {
		text = cleanPreviewText(article.Excerpt())
	}
	if text == "" {
		text = strings.TrimSpace(title)
	}
	if text == "" {
		return "", fmt.Errorf("reader extracted empty content")
	}

	return text, nil
}

func cleanPreviewText(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	lines := strings.Split(normalized, "\n")
	paragraphs := make([]string, 0, len(lines))
	for _, line := range lines {
		clean := strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if clean == "" {
			continue
		}
		paragraphs = append(paragraphs, clean)
	}

	return strings.TrimSpace(strings.Join(paragraphs, "\n\n"))
}

func truncatePreviewText(raw string, maxChars int) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}

	limit := maxChars
	if limit <= 0 {
		limit = defaultArticlePreviewMaxChars
	}

	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed, false
	}
	if limit == 1 {
		return "…", true
	}

	clipped := strings.TrimSpace(string(runes[:limit-1]))
	if clipped == "" {
		return "…", true
	}
	return clipped + "…", true
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
