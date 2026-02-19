package reader

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
)

const (
	DefaultFetchTimeout  = 12 * time.Second
	DefaultBodyByteLimit = 2 * 1024 * 1024

	defaultUserAgent = "SCOOP-ReaderPreview/1.0 (+https://github.com/janitrai/scoop)"
)

// FetchOptions controls HTTP behavior for reader preview extraction.
type FetchOptions struct {
	Timeout       time.Duration
	BodyByteLimit int64
	UserAgent     string
	HTTPClient    *http.Client
}

// FetchText retrieves and extracts readable text content for a canonical URL.
func FetchText(ctx context.Context, canonicalURL string, title string) (string, error) {
	return FetchTextWithOptions(ctx, canonicalURL, title, FetchOptions{})
}

// FetchTextWithOptions retrieves and extracts readable text content for a canonical URL.
func FetchTextWithOptions(ctx context.Context, canonicalURL string, title string, opts FetchOptions) (string, error) {
	page := strings.TrimSpace(canonicalURL)
	if page == "" {
		return "", fmt.Errorf("canonical URL is required")
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultFetchTimeout
	}

	bodyLimit := opts.BodyByteLimit
	if bodyLimit <= 0 {
		bodyLimit = DefaultBodyByteLimit
	}

	fetchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, page, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	userAgent := strings.TrimSpace(opts.UserAgent)
	if userAgent == "" {
		userAgent = defaultUserAgent
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.8")

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch url: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, bodyLimit))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "text/plain") {
		return CleanText(string(body)), nil
	}

	pageURL, err := url.Parse(page)
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

	text := CleanText(renderedText.String())
	if text == "" {
		text = CleanText(article.Excerpt())
	}
	if text == "" {
		text = strings.TrimSpace(title)
	}
	if text == "" {
		return "", fmt.Errorf("reader extracted empty content")
	}

	return text, nil
}

// CleanText normalizes line endings and collapses extra in-line whitespace.
func CleanText(raw string) string {
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

// TruncateText clips text to maxChars runes and appends a single ellipsis rune when truncated.
func TruncateText(raw string, maxChars int) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	if maxChars <= 0 {
		return trimmed, false
	}

	runes := []rune(trimmed)
	if len(runes) <= maxChars {
		return trimmed, false
	}
	if maxChars == 1 {
		return "…", true
	}

	clipped := strings.TrimSpace(string(runes[:maxChars-1]))
	if clipped == "" {
		return "…", true
	}

	return clipped + "…", true
}
