package translation

import "testing"

func TestShouldSkipTranslationTask(t *testing.T) {
	t.Parallel()

	if !shouldSkipTranslationTask("en", "en") {
		t.Fatalf("expected same language pair to be skipped")
	}
	if shouldSkipTranslationTask("und", "en") {
		t.Fatalf("did not expect und->en to be skipped")
	}
	if shouldSkipTranslationTask("", "en") {
		t.Fatalf("did not expect empty source language to be skipped")
	}
}
