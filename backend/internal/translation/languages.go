package translation

import (
	"sort"
	"strings"
)

type LanguageOption struct {
	Code   string `json:"code"`
	Label  string `json:"label"`
	Native string `json:"native,omitempty"`
}

func ViewerLanguageOptions(registry *Registry) []LanguageOption {
	options := []LanguageOption{
		{
			Code:  "original",
			Label: "Original",
		},
	}
	options = append(options, TranslationLanguageOptions(registry)...)
	return options
}

func TranslationLanguageOptions(registry *Registry) []LanguageOption {
	supported := map[string]struct{}{}

	for code := range translationLangLabels {
		normalized := normalizeLangCode(code)
		if normalized == "" {
			continue
		}
		supported[normalized] = struct{}{}
	}

	if registry != nil {
		for _, provider := range registry.providers {
			for _, code := range provider.SupportedLanguages() {
				normalized := normalizeLangCode(code)
				if normalized == "" {
					continue
				}
				supported[normalized] = struct{}{}
			}
		}
	}

	codes := make([]string, 0, len(supported))
	for code := range supported {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	options := make([]LanguageOption, 0, len(codes))
	for _, code := range codes {
		labels, hasLabels := translationLangLabels[code]
		if hasLabels {
			options = append(options, LanguageOption{
				Code:   code,
				Label:  labels.english,
				Native: labels.chinese,
			})
			continue
		}

		options = append(options, LanguageOption{
			Code:  code,
			Label: strings.ToUpper(code),
		})
	}

	return options
}
