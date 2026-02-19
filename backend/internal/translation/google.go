package translation

import (
	"context"
	"fmt"
)

// GoogleProvider is a placeholder for Google Cloud Translation API integration.
type GoogleProvider struct{}

func NewGoogleProvider() *GoogleProvider {
	return &GoogleProvider{}
}

func (p *GoogleProvider) Name() string {
	return "google"
}

func (p *GoogleProvider) SupportedLanguages() []string {
	return []string{}
}

func (p *GoogleProvider) Translate(ctx context.Context, req TranslateRequest) (*TranslateResponse, error) {
	_ = ctx
	_ = req
	// TODO: Implement Google Cloud Translation API integration.
	return nil, fmt.Errorf("google translation provider is not implemented")
}
