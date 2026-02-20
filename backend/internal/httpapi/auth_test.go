package httpapi

import (
	"testing"

	"horse.fit/scoop/internal/translation"
)

func TestNormalizeViewerLanguage(t *testing.T) {
	t.Parallel()

	if got := normalizeViewerLanguage(" EN-us "); got != "en-us" {
		t.Fatalf("unexpected normalized language: %q", got)
	}
	if got := normalizeViewerLanguage(""); got != defaultViewerLanguage {
		t.Fatalf("expected default language, got %q", got)
	}
}

func TestIsSupportedViewerLanguage(t *testing.T) {
	t.Parallel()

	options := []translation.LanguageOption{
		{Code: "original", Label: "Original"},
		{Code: "en", Label: "English"},
	}

	if !isSupportedViewerLanguage("en", options) {
		t.Fatalf("expected en to be supported")
	}
	if !isSupportedViewerLanguage("original", options) {
		t.Fatalf("expected original to be supported")
	}
	if isSupportedViewerLanguage("xx", options) {
		t.Fatalf("did not expect xx to be supported")
	}
}
