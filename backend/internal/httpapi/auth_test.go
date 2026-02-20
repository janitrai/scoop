package httpapi

import (
	"testing"
	"time"

	"github.com/rs/zerolog"

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

func TestSessionExpiryUsesConfiguredTTL(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC)
	server := &Server{
		opts: Options{
			SessionTTL: 6 * time.Hour,
		},
	}

	got := server.sessionExpiry(now)
	want := now.Add(6 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("unexpected session expiry: got %s want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestNewServerSetsDefaultSessionTTL(t *testing.T) {
	t.Parallel()

	server := NewServer(nil, zerolog.Nop(), Options{})
	if server == nil {
		t.Fatalf("expected server")
	}
	if server.opts.SessionTTL != 7*24*time.Hour {
		t.Fatalf("unexpected default session ttl: got %s", server.opts.SessionTTL)
	}
}
