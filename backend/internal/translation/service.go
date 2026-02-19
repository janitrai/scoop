package translation

import "context"

// Provider translates free-form text between languages.
type Provider interface {
	Translate(ctx context.Context, req TranslateRequest) (*TranslateResponse, error)
	Name() string
	SupportedLanguages() []string
}

// TranslateRequest describes one translation request.
type TranslateRequest struct {
	Text       string
	SourceLang string // ISO 639-1 (for example: "zh", "en")
	TargetLang string
}

// TranslateResponse contains translated text and provider metadata.
type TranslateResponse struct {
	Text         string
	SourceLang   string
	TargetLang   string
	ProviderName string
	LatencyMs    int64
}
