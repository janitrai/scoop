package language

import "testing"

func TestNormalizeTag(t *testing.T) {
	t.Parallel()

	if got := NormalizeTag(" EN_us "); got != "en-us" {
		t.Fatalf("unexpected normalized tag: %q", got)
	}
	if got := NormalizeTag("zh-Hans"); got != "zh-hans" {
		t.Fatalf("unexpected normalized tag: %q", got)
	}
	if got := NormalizeTag("en--US"); got != "en-us" {
		t.Fatalf("unexpected collapsed tag: %q", got)
	}
	if got := NormalizeTag("en_123"); got != "" {
		t.Fatalf("expected invalid tag to normalize to empty string, got %q", got)
	}
}

func TestNormalizeCode(t *testing.T) {
	t.Parallel()

	if got := NormalizeCode(" EN-us "); got != "en" {
		t.Fatalf("unexpected normalized code: %q", got)
	}
	if got := NormalizeCode("zh"); got != "zh" {
		t.Fatalf("unexpected normalized code: %q", got)
	}
	if got := NormalizeCode(" "); got != "" {
		t.Fatalf("expected empty code for blank input, got %q", got)
	}
}
