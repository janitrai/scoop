package httpapi

import (
	"encoding/json"
	"strings"

	"github.com/labstack/echo/v4"

	"horse.fit/scoop/internal/db"
	"horse.fit/scoop/internal/translation"
)

const defaultViewerLanguage = "en"

type userSettingsResponse struct {
	PreferredLanguage string         `json:"preferred_language"`
	UIPrefs           map[string]any `json:"ui_prefs"`
}

func (s *Server) handleGetMySettings(c echo.Context) error {
	principal, ok := principalFromContext(c)
	if !ok {
		return unauthorizedResponse(c)
	}

	settings, err := s.pool.EnsureUserSettings(c.Request().Context(), principal.UserID)
	if err != nil {
		s.logger.Error().Err(err).Int64("user_id", principal.UserID).Msg("query user settings failed")
		return internalError(c, "Failed to load user settings")
	}

	return success(c, map[string]any{
		"settings": buildSettingsResponse(settings),
	})
}

func (s *Server) handlePutMySettings(c echo.Context) error {
	principal, ok := principalFromContext(c)
	if !ok {
		return unauthorizedResponse(c)
	}

	var payload map[string]json.RawMessage
	if err := decodeJSONBody(c, &payload); err != nil {
		return failValidation(c, map[string]string{"body": err.Error()})
	}
	if len(payload) == 0 {
		return failValidation(c, map[string]string{"body": "at least one settings field is required"})
	}
	for key := range payload {
		switch key {
		case "preferred_language", "ui_prefs":
			// Supported.
		default:
			return failValidation(c, map[string]string{key: "is not a supported settings field"})
		}
	}

	current, err := s.pool.EnsureUserSettings(c.Request().Context(), principal.UserID)
	if err != nil {
		s.logger.Error().Err(err).Int64("user_id", principal.UserID).Msg("load current settings failed")
		return internalError(c, "Failed to load user settings")
	}

	preferredLanguage := normalizeViewerLanguage(current.PreferredLanguage)
	uiPrefs := append(json.RawMessage(nil), current.UIPrefs...)
	if len(uiPrefs) == 0 {
		uiPrefs = json.RawMessage(`{}`)
	}

	if rawLang, exists := payload["preferred_language"]; exists {
		var langInput string
		if err := json.Unmarshal(rawLang, &langInput); err != nil {
			return failValidation(c, map[string]string{"preferred_language": "must be a string"})
		}
		preferredLanguage = normalizeViewerLanguage(langInput)
		if !isSupportedViewerLanguage(preferredLanguage, s.viewerLanguageOptions()) {
			return failValidation(c, map[string]string{"preferred_language": "is not supported"})
		}
	}

	if rawPrefs, exists := payload["ui_prefs"]; exists {
		trimmed := strings.TrimSpace(string(rawPrefs))
		if trimmed == "" || trimmed == "null" {
			uiPrefs = json.RawMessage(`{}`)
		} else {
			var asMap map[string]any
			if err := json.Unmarshal(rawPrefs, &asMap); err != nil {
				return failValidation(c, map[string]string{"ui_prefs": "must be a JSON object"})
			}
			normalized, err := json.Marshal(asMap)
			if err != nil {
				return internalError(c, "Failed to persist ui_prefs")
			}
			uiPrefs = normalized
		}
	}

	updated, err := s.pool.UpsertUserSettings(c.Request().Context(), principal.UserID, preferredLanguage, uiPrefs)
	if err != nil {
		s.logger.Error().Err(err).Int64("user_id", principal.UserID).Msg("update user settings failed")
		return internalError(c, "Failed to update user settings")
	}

	return success(c, map[string]any{
		"settings": buildSettingsResponse(updated),
	})
}

func (s *Server) handleLanguages(c echo.Context) error {
	return success(c, map[string]any{
		"items": s.viewerLanguageOptions(),
	})
}

func (s *Server) viewerLanguageOptions() []translation.LanguageOption {
	if s == nil {
		return translation.ViewerLanguageOptions(nil)
	}
	return translation.ViewerLanguageOptions(s.registry)
}

func buildSettingsResponse(row *db.UserSettingsRecord) userSettingsResponse {
	if row == nil {
		return userSettingsResponse{
			PreferredLanguage: defaultViewerLanguage,
			UIPrefs:           map[string]any{},
		}
	}

	uiPrefs := map[string]any{}
	if len(row.UIPrefs) > 0 {
		_ = json.Unmarshal(row.UIPrefs, &uiPrefs)
	}

	return userSettingsResponse{
		PreferredLanguage: normalizeViewerLanguage(row.PreferredLanguage),
		UIPrefs:           uiPrefs,
	}
}

func normalizeViewerLanguage(raw string) string {
	lang := strings.ToLower(strings.TrimSpace(raw))
	if lang == "" {
		return defaultViewerLanguage
	}
	lang = strings.ReplaceAll(lang, "_", "-")
	return lang
}

func isSupportedViewerLanguage(lang string, options []translation.LanguageOption) bool {
	normalized := normalizeViewerLanguage(lang)
	for _, option := range options {
		if normalizeViewerLanguage(option.Code) == normalized {
			return true
		}
	}
	return false
}
