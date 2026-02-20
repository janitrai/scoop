package language

import "strings"

// NormalizeTag normalizes a language tag to lowercase and "-" separators.
// Returns an empty string when the value is blank or contains invalid characters.
func NormalizeTag(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return ""
	}

	trimmed = strings.ReplaceAll(trimmed, "_", "-")
	parts := strings.Split(trimmed, "-")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !isAlphaLower(part) {
			return ""
		}
		normalized = append(normalized, part)
	}

	if len(normalized) == 0 {
		return ""
	}
	return strings.Join(normalized, "-")
}

// NormalizeCode returns the primary language subtag (for example, "en" from "en-US").
func NormalizeCode(raw string) string {
	tag := NormalizeTag(raw)
	if tag == "" {
		return ""
	}
	if dash := strings.IndexByte(tag, '-'); dash >= 0 {
		return tag[:dash]
	}
	return tag
}

func isAlphaLower(value string) bool {
	for _, r := range value {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}
