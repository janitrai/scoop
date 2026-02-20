package translation

import "horse.fit/scoop/internal/language"

func normalizeLangCode(raw string) string {
	return language.NormalizeCode(raw)
}
