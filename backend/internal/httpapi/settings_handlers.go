package httpapi

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/labstack/echo/v4"

	"horse.fit/scoop/internal/auth"
	"horse.fit/scoop/internal/db"
	"horse.fit/scoop/internal/language"
	"horse.fit/scoop/internal/translation"
)

const defaultViewerLanguage = "en"
const passwordEnabledUIPrefKey = "password_enabled"

type userSettingsResponse struct {
	PreferredLanguage string         `json:"preferred_language"`
	PasswordEnabled   bool           `json:"password_enabled"`
	UIPrefs           map[string]any `json:"ui_prefs"`
}

func (s *Server) handleGetMySettings(c echo.Context) error {
	store := s.authDataStore()
	if store == nil {
		return internalError(c, "Failed to load user settings")
	}

	principal, ok := principalFromContext(c)
	if !ok {
		return unauthorizedResponse(c)
	}

	settings, err := store.EnsureUserSettings(c.Request().Context(), principal.UserID)
	if err != nil {
		s.logger.Error().Err(err).Int64("user_id", principal.UserID).Msg("query user settings failed")
		return internalError(c, "Failed to load user settings")
	}

	return success(c, map[string]any{
		"settings": buildSettingsResponse(settings),
	})
}

func (s *Server) handlePutMySettings(c echo.Context) error {
	store := s.authDataStore()
	if store == nil {
		return internalError(c, "Failed to load user settings")
	}

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
		case "preferred_language", "ui_prefs", "password_enabled", "password":
			// Supported.
		default:
			return failValidation(c, map[string]string{key: "is not a supported settings field"})
		}
	}

	current, err := store.EnsureUserSettings(c.Request().Context(), principal.UserID)
	if err != nil {
		s.logger.Error().Err(err).Int64("user_id", principal.UserID).Msg("load current settings failed")
		return internalError(c, "Failed to load user settings")
	}

	preferredLanguage := normalizeViewerLanguage(current.PreferredLanguage)
	uiPrefsMap := decodeUIPrefs(current.UIPrefs)
	passwordEnabled := isPasswordEnabledMap(uiPrefsMap)
	currentPasswordEnabled := passwordEnabled

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
			uiPrefsMap = map[string]any{}
		} else {
			var asMap map[string]any
			if err := json.Unmarshal(rawPrefs, &asMap); err != nil {
				return failValidation(c, map[string]string{"ui_prefs": "must be a JSON object"})
			}
			uiPrefsMap = asMap
		}
		passwordEnabled = isPasswordEnabledMap(uiPrefsMap)
	}

	if rawPasswordEnabled, exists := payload["password_enabled"]; exists {
		var enabled bool
		if err := json.Unmarshal(rawPasswordEnabled, &enabled); err != nil {
			return failValidation(c, map[string]string{"password_enabled": "must be a boolean"})
		}
		passwordEnabled = enabled
	}

	passwordProvided := false
	if rawPassword, exists := payload["password"]; exists {
		var password string
		if err := json.Unmarshal(rawPassword, &password); err != nil {
			return failValidation(c, map[string]string{"password": "must be a string"})
		}
		password = strings.TrimSpace(password)
		if password == "" {
			return failValidation(c, map[string]string{"password": "is required"})
		}

		passwordHash, err := auth.HashPassword(password)
		if err != nil {
			return internalError(c, "Failed to update password")
		}
		if err := store.SetUserPasswordHash(c.Request().Context(), principal.UserID, passwordHash, false); err != nil {
			if errors.Is(err, db.ErrNoRows) {
				return unauthorizedResponse(c)
			}
			s.logger.Error().Err(err).Int64("user_id", principal.UserID).Msg("update user password failed")
			return internalError(c, "Failed to update password")
		}
		passwordProvided = true
	}

	if passwordEnabled && !currentPasswordEnabled && !passwordProvided {
		return failValidation(c, map[string]string{"password": "is required when enabling password authentication"})
	}

	uiPrefsMap[passwordEnabledUIPrefKey] = passwordEnabled
	uiPrefs, err := json.Marshal(uiPrefsMap)
	if err != nil {
		return internalError(c, "Failed to persist ui_prefs")
	}

	updated, err := store.UpsertUserSettings(c.Request().Context(), principal.UserID, preferredLanguage, uiPrefs)
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
			PasswordEnabled:   false,
			UIPrefs:           map[string]any{},
		}
	}

	uiPrefs := decodeUIPrefs(row.UIPrefs)

	return userSettingsResponse{
		PreferredLanguage: normalizeViewerLanguage(row.PreferredLanguage),
		PasswordEnabled:   isPasswordEnabledMap(uiPrefs),
		UIPrefs:           uiPrefs,
	}
}

func normalizeViewerLanguage(raw string) string {
	lang := language.NormalizeTag(raw)
	if lang == "" {
		return defaultViewerLanguage
	}
	if lang == "original" {
		return "original"
	}
	lang = language.NormalizeCode(lang)
	if lang == "" {
		return defaultViewerLanguage
	}
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

func decodeUIPrefs(raw json.RawMessage) map[string]any {
	uiPrefs := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &uiPrefs)
	}
	if uiPrefs == nil {
		uiPrefs = map[string]any{}
	}
	return uiPrefs
}

func isPasswordEnabled(settings *db.UserSettingsRecord) bool {
	if settings == nil {
		return false
	}
	return isPasswordEnabledMap(decodeUIPrefs(settings.UIPrefs))
}

func isPasswordEnabledMap(uiPrefs map[string]any) bool {
	if len(uiPrefs) == 0 {
		return false
	}

	raw, exists := uiPrefs[passwordEnabledUIPrefKey]
	if !exists {
		return false
	}

	switch value := raw.(type) {
	case bool:
		return value
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "1", "yes":
			return true
		default:
			return false
		}
	case float64:
		return value == 1
	default:
		return false
	}
}
