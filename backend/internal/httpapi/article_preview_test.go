package httpapi

import (
	"context"
	"testing"
)

func TestCleanPreviewTextCollapsesWhitespaceAndPreservesParagraphs(t *testing.T) {
	input := "  First   paragraph \n\n Second\tparagraph \r\n\r\nThird line "
	got := cleanPreviewText(input)
	want := "First paragraph\n\nSecond paragraph\n\nThird line"
	if got != want {
		t.Fatalf("cleanPreviewText mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestTruncatePreviewText(t *testing.T) {
	input := "abcdefghijklmnopqrstuvwxyz"

	got, truncated := truncatePreviewText(input, 10)
	if !truncated {
		t.Fatalf("expected truncated=true")
	}
	if got != "abcdefghi…" {
		t.Fatalf("unexpected truncated text: %q", got)
	}

	full, wasTruncated := truncatePreviewText("short", 10)
	if wasTruncated {
		t.Fatalf("expected truncated=false for short text")
	}
	if full != "short" {
		t.Fatalf("unexpected short text: %q", full)
	}
}

func TestBuildArticlePreviewTextFallsBackToNormalizedTextWhenNoURL(t *testing.T) {
	text, source, err := buildArticlePreviewText(context.Background(), nil, "title", "normalized body")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if source != "normalized_text" {
		t.Fatalf("unexpected source: %q", source)
	}
	if text != "normalized body" {
		t.Fatalf("unexpected text: %q", text)
	}
}
