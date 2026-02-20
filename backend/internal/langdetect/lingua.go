package langdetect

import (
	"strings"
	"sync"
	"unicode"

	lingua "github.com/pemistahl/lingua-go"
)

var (
	detectorOnce sync.Once
	detector     lingua.LanguageDetector
)

func DetectISO6391(text string) string {
	sample := strings.TrimSpace(text)
	if sample == "" {
		return ""
	}

	letterCount := 0
	for _, r := range sample {
		if unicode.IsLetter(r) {
			letterCount++
		}
	}
	if letterCount < 6 {
		return ""
	}

	language, exists := getDetector().DetectLanguageOf(sample)
	if !exists {
		return ""
	}

	code := strings.ToLower(language.IsoCode639_1().String())
	if len(code) != 2 {
		return ""
	}
	return code
}

func getDetector() lingua.LanguageDetector {
	detectorOnce.Do(func() {
		detector = lingua.NewLanguageDetectorBuilder().
			FromAllLanguages().
			WithPreloadedLanguageModels().
			Build()
	})
	return detector
}
