package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"horse.fit/scoop/internal/auth"
	"horse.fit/scoop/internal/db"
	"horse.fit/scoop/internal/globaltime"
	"horse.fit/scoop/internal/translation"
)

const (
	defaultSessionTouchInterval = time.Minute
	defaultViewerLanguage       = "original"
)

type authPrincipal struct {
	SessionID          string
	UserID             int64
	Username           string
	MustChangePassword bool
	ExpiresAt          time.Time
}

type authUserResponse struct {
	UserID             int64      `json:"user_id"`
	Username           string     `json:"username"`
	MustChangePassword bool       `json:"must_change_password"`
	CreatedAt          time.Time  `json:"created_at"`
	LastLoginAt        *time.Time `json:"last_login_at,omitempty"`
}

type userSettingsResponse struct {
	PreferredLanguage string         `json:"preferred_language"`
	UIPrefs           map[string]any `json:"ui_prefs"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) requireAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c == nil {
				return unauthorizedResponse(c)
			}

			sessionID, found := s.sessionIDFromCookie(c)
			if !found {
				return unauthorizedResponse(c)
			}

			session, err := s.pool.GetSession(c.Request().Context(), sessionID)
			if err != nil {
				if errors.Is(err, db.ErrNoRows) {
					s.clearSessionCookie(c)
					return unauthorizedResponse(c)
				}
				s.logger.Error().Err(err).Msg("session lookup failed")
				return internalError(c, "Failed to authorize request")
			}

			now := globaltime.UTC()
			if !session.ExpiresAt.After(now) {
				_ = s.pool.DeleteSession(c.Request().Context(), session.SessionID)
				s.clearSessionCookie(c)
				return unauthorizedResponse(c)
			}

			if now.Sub(session.LastSeenAt) >= defaultSessionTouchInterval {
				_ = s.pool.TouchSession(c.Request().Context(), session.SessionID, now)
			}

			c.Set("auth.principal", authPrincipal{
				SessionID:          session.SessionID,
				UserID:             session.UserID,
				Username:           session.Username,
				MustChangePassword: session.MustChangePassword,
				ExpiresAt:          session.ExpiresAt.UTC(),
			})

			return next(c)
		}
	}
}

func (s *Server) handleLogin(c echo.Context) error {
	var req loginRequest
	if err := decodeJSONBody(c, &req); err != nil {
		return failValidation(c, map[string]string{"body": err.Error()})
	}

	username := auth.NormalizeUsername(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		return failValidation(c, map[string]string{
			"username": "is required",
			"password": "is required",
		})
	}

	user, err := s.pool.GetUserByUsername(c.Request().Context(), username)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return fail(c, http.StatusUnauthorized, "Invalid username or password", nil)
		}
		s.logger.Error().Err(err).Str("username", username).Msg("login lookup failed")
		return internalError(c, "Failed to process login")
	}

	if !auth.VerifyPassword(password, user.PasswordHash) {
		return fail(c, http.StatusUnauthorized, "Invalid username or password", nil)
	}

	now := globaltime.UTC()
	expiresAt := now.Add(s.opts.SessionTTL)
	sessionID, err := s.pool.CreateSession(c.Request().Context(), user.UserID, expiresAt, now)
	if err != nil {
		s.logger.Error().Err(err).Int64("user_id", user.UserID).Msg("create session failed")
		return internalError(c, "Failed to process login")
	}

	if err := s.pool.SetUserLastLogin(c.Request().Context(), user.UserID, now); err != nil {
		s.logger.Error().Err(err).Int64("user_id", user.UserID).Msg("update last login failed")
	}
	nowCopy := now
	user.LastLoginAt = &nowCopy

	settings, err := s.pool.EnsureUserSettings(c.Request().Context(), user.UserID)
	if err != nil {
		s.logger.Error().Err(err).Int64("user_id", user.UserID).Msg("ensure settings failed")
		return internalError(c, "Failed to load settings")
	}

	s.setSessionCookie(c, sessionID, expiresAt)
	return success(c, map[string]any{
		"user":      buildAuthUserResponse(user),
		"settings":  buildSettingsResponse(settings),
		"languages": translation.ViewerLanguageOptions(s.registry),
		"session": map[string]any{
			"session_id": sessionID,
			"expires_at": expiresAt.UTC(),
		},
	})
}

func (s *Server) handleLogout(c echo.Context) error {
	if sessionID, found := s.sessionIDFromCookie(c); found {
		_ = s.pool.DeleteSession(c.Request().Context(), sessionID)
	}
	s.clearSessionCookie(c)
	return success(c, map[string]any{"logged_out": true})
}

func (s *Server) handleMe(c echo.Context) error {
	principal, ok := principalFromContext(c)
	if !ok {
		return unauthorizedResponse(c)
	}

	user, err := s.pool.GetUserByID(c.Request().Context(), principal.UserID)
	if err != nil {
		if errors.Is(err, db.ErrNoRows) {
			return unauthorizedResponse(c)
		}
		s.logger.Error().Err(err).Int64("user_id", principal.UserID).Msg("load me user failed")
		return internalError(c, "Failed to load user")
	}

	settings, err := s.pool.EnsureUserSettings(c.Request().Context(), principal.UserID)
	if err != nil {
		s.logger.Error().Err(err).Int64("user_id", principal.UserID).Msg("load me settings failed")
		return internalError(c, "Failed to load settings")
	}

	return success(c, map[string]any{
		"user":      buildAuthUserResponse(user),
		"settings":  buildSettingsResponse(settings),
		"languages": translation.ViewerLanguageOptions(s.registry),
	})
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
		if !isSupportedViewerLanguage(preferredLanguage, translation.ViewerLanguageOptions(s.registry)) {
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
		"items": translation.ViewerLanguageOptions(s.registry),
	})
}

func unauthorizedResponse(c echo.Context) error {
	if c == nil {
		return fmt.Errorf("authentication required")
	}
	return fail(c, http.StatusUnauthorized, "Authentication required", nil)
}

func buildAuthUserResponse(row *db.AuthUser) authUserResponse {
	if row == nil {
		return authUserResponse{}
	}
	return authUserResponse{
		UserID:             row.UserID,
		Username:           row.Username,
		MustChangePassword: row.MustChangePassword,
		CreatedAt:          row.CreatedAt.UTC(),
		LastLoginAt:        row.LastLoginAt,
	}
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

func principalFromContext(c echo.Context) (authPrincipal, bool) {
	if c == nil {
		return authPrincipal{}, false
	}
	value := c.Get("auth.principal")
	principal, ok := value.(authPrincipal)
	if !ok {
		return authPrincipal{}, false
	}
	return principal, true
}

func (s *Server) sessionIDFromCookie(c echo.Context) (string, bool) {
	if c == nil {
		return "", false
	}

	cookie, err := c.Cookie(s.opts.SessionCookie)
	if err != nil || cookie == nil {
		return "", false
	}

	sessionID := strings.TrimSpace(cookie.Value)
	if sessionID == "" {
		return "", false
	}
	return sessionID, true
}

func (s *Server) setSessionCookie(c echo.Context, sessionID string, expiresAt time.Time) {
	if c == nil {
		return
	}

	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}

	c.SetCookie(&http.Cookie{
		Name:     s.opts.SessionCookie,
		Value:    strings.TrimSpace(sessionID),
		Path:     "/",
		HttpOnly: true,
		Secure:   s.opts.SessionSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt.UTC(),
		MaxAge:   maxAge,
	})
}

func (s *Server) clearSessionCookie(c echo.Context) {
	if c == nil {
		return
	}

	c.SetCookie(&http.Cookie{
		Name:     s.opts.SessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.opts.SessionSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  globaltime.UTC().Add(-1 * time.Hour),
	})
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
