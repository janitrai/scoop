package reader

import "testing"

func TestCleanTextCollapsesWhitespaceAndPreservesParagraphs(t *testing.T) {
	input := "  First   paragraph \n\n Second\tparagraph \r\n\r\nThird line "
	got := CleanText(input)
	want := "First paragraph\n\nSecond paragraph\n\nThird line"
	if got != want {
		t.Fatalf("CleanText mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestTruncateText(t *testing.T) {
	input := "abcdefghijklmnopqrstuvwxyz"

	got, truncated := TruncateText(input, 10)
	if !truncated {
		t.Fatalf("expected truncated=true")
	}
	if got != "abcdefghi…" {
		t.Fatalf("unexpected truncated text: %q", got)
	}

	full, wasTruncated := TruncateText("short", 10)
	if wasTruncated {
		t.Fatalf("expected truncated=false for short text")
	}
	if full != "short" {
		t.Fatalf("unexpected short text: %q", full)
	}
}
