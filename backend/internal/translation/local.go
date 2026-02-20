package translation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// DefaultLocalEndpoint points to a local OpenAI-compatible translation endpoint.
	DefaultLocalEndpoint = "http://127.0.0.1:8845/v1"
	// DefaultLocalModel is the default HY-MT model name.
	DefaultLocalModel = "tencent/HY-MT1.5-7B"
)

// LocalProvider translates text by calling an OpenAI-compatible chat completions endpoint.
type LocalProvider struct {
	endpointURL string
	model       string
	client      *http.Client
}

// NewLocalProviderFromEnv builds a local provider from env vars.
//   - TRANSLATION_ENDPOINT (default: http://127.0.0.1:8845/v1)
//   - TRANSLATION_MODEL (default: tencent/HY-MT1.5-7B)
func NewLocalProviderFromEnv() *LocalProvider {
	endpoint := strings.TrimSpace(os.Getenv("TRANSLATION_ENDPOINT"))
	if endpoint == "" {
		endpoint = DefaultLocalEndpoint
	}
	model := strings.TrimSpace(os.Getenv("TRANSLATION_MODEL"))
	if model == "" {
		model = DefaultLocalModel
	}
	return NewLocalProvider(endpoint, model)
}

// NewLocalProvider builds a local provider for the given endpoint/model.
func NewLocalProvider(endpoint, model string) *LocalProvider {
	normalizedEndpoint := normalizeEndpoint(endpoint)
	trimmedModel := strings.TrimSpace(model)
	if trimmedModel == "" {
		trimmedModel = DefaultLocalModel
	}
	return &LocalProvider{
		endpointURL: chatCompletionsURL(normalizedEndpoint),
		model:       trimmedModel,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (p *LocalProvider) Name() string {
	return "local"
}

// ModelName returns the configured model identifier.
func (p *LocalProvider) ModelName() string {
	if p == nil {
		return ""
	}
	return p.model
}

func (p *LocalProvider) SupportedLanguages() []string {
	return SupportedTranslationLanguageCodes()
}

func (p *LocalProvider) Translate(ctx context.Context, req TranslateRequest) (*TranslateResponse, error) {
	if p == nil {
		return nil, fmt.Errorf("local provider is nil")
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	sourceLang := normalizeLangCode(req.SourceLang)
	targetLang := normalizeLangCode(req.TargetLang)
	if targetLang == "" {
		return nil, fmt.Errorf("target language is required")
	}

	prompt := buildHYMTPrompt(text, sourceLang, targetLang)
	body, err := json.Marshal(localChatRequest{
		Model: p.model,
		Messages: []localChatMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.7,
		TopP:        0.6,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal translation request: %w", err)
	}

	started := time.Now()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpointURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build translation request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send translation request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read translation response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errPayload localChatErrorResponse
		if unmarshalErr := json.Unmarshal(respBody, &errPayload); unmarshalErr == nil {
			if msg := strings.TrimSpace(errPayload.Error.Message); msg != "" {
				return nil, fmt.Errorf("translation endpoint status %d: %s", resp.StatusCode, msg)
			}
		}
		return nil, fmt.Errorf("translation endpoint status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed localChatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode translation response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("translation response missing choices")
	}

	translated := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if translated == "" {
		return nil, fmt.Errorf("translation response was empty")
	}

	latency := time.Since(started).Milliseconds()
	return &TranslateResponse{
		Text:         translated,
		SourceLang:   sourceLang,
		TargetLang:   targetLang,
		ProviderName: p.Name(),
		LatencyMs:    latency,
	}, nil
}

type localChatRequest struct {
	Model       string             `json:"model"`
	Messages    []localChatMessage `json:"messages"`
	Temperature float64            `json:"temperature,omitempty"`
	TopP        float64            `json:"top_p,omitempty"`
}

type localChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type localChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type localChatErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func buildHYMTPrompt(text, sourceLang, targetLang string) string {
	target := targetLanguageLabel(targetLang)
	if isChineseLanguage(sourceLang) || isChineseLanguage(targetLang) {
		// HY-MT zh<=>xx template.
		return fmt.Sprintf("将以下文本翻译为%s，注意只需要输出翻译后的结果，不要额外解释：\n\n%s", target.chinese, text)
	}
	// HY-MT xx<=>xx template.
	return fmt.Sprintf("Translate the following segment into %s, without additional explanation.\n\n%s", target.english, text)
}

func targetLanguageLabel(lang string) languageLabel {
	normalized := normalizeLangCode(lang)
	if labels, ok := translationLanguageLabels[normalized]; ok {
		return labels
	}
	fallback := strings.TrimSpace(lang)
	if fallback == "" {
		fallback = "English"
	}
	return languageLabel{english: fallback, chinese: fallback}
}

func isChineseLanguage(lang string) bool {
	return normalizeLangCode(lang) == "zh"
}

func normalizeEndpoint(raw string) string {
	endpoint := strings.TrimSpace(raw)
	if endpoint == "" {
		return DefaultLocalEndpoint
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}

	parsed, err := url.Parse(endpoint)
	if err != nil || strings.TrimSpace(parsed.Host) == "" {
		return DefaultLocalEndpoint
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if parsed.Path == "" {
		parsed.Path = "/v1"
	}
	return parsed.String()
}

func chatCompletionsURL(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil || strings.TrimSpace(parsed.Host) == "" {
		return DefaultLocalEndpoint + "/chat/completions"
	}

	path := strings.TrimRight(parsed.Path, "/")
	switch {
	case strings.HasSuffix(path, "/chat/completions"):
		parsed.Path = path
	case strings.HasSuffix(path, "/v1"):
		parsed.Path = path + "/chat/completions"
	case path == "":
		parsed.Path = "/v1/chat/completions"
	default:
		parsed.Path = path + "/v1/chat/completions"
	}

	return parsed.String()
}
