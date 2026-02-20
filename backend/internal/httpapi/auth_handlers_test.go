package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"horse.fit/scoop/internal/auth"
	"horse.fit/scoop/internal/db"
)

type upsertSettingsCall struct {
	userID            int64
	preferredLanguage string
	uiPrefs           []byte
}

type setPasswordCall struct {
	userID             int64
	passwordHash       string
	mustChangePassword bool
}

type fakeAuthStore struct {
	sessions           map[string]*db.AuthSession
	usersByUsername    map[string]*db.AuthUser
	usersByID          map[int64]*db.AuthUser
	settingsByUserID   map[int64]*db.UserSettingsRecord
	createSessionID    string
	createSessionCalls int
	deleteSessionCalls []string
	getSessionCalls    int
	touchSessionCalls  int
	setLastLoginCalls  int
	deleteExpiredCalls int
	upsertCalls        []upsertSettingsCall
	setPasswordCalls   []setPasswordCall
}

func newFakeAuthStore() *fakeAuthStore {
	return &fakeAuthStore{
		sessions:         map[string]*db.AuthSession{},
		usersByUsername:  map[string]*db.AuthUser{},
		usersByID:        map[int64]*db.AuthUser{},
		settingsByUserID: map[int64]*db.UserSettingsRecord{},
	}
}

func (s *fakeAuthStore) GetSession(_ context.Context, sessionID string) (*db.AuthSession, error) {
	s.getSessionCalls++
	row, exists := s.sessions[sessionID]
	if !exists {
		return nil, db.ErrNoRows
	}
	copyRow := *row
	return &copyRow, nil
}

func (s *fakeAuthStore) DeleteSession(_ context.Context, sessionID string) error {
	s.deleteSessionCalls = append(s.deleteSessionCalls, sessionID)
	delete(s.sessions, sessionID)
	return nil
}

func (s *fakeAuthStore) TouchSession(_ context.Context, sessionID string, seenAt time.Time) error {
	s.touchSessionCalls++
	row, exists := s.sessions[sessionID]
	if !exists {
		return db.ErrNoRows
	}
	row.LastSeenAt = seenAt
	return nil
}

func (s *fakeAuthStore) GetUserByUsername(_ context.Context, username string) (*db.AuthUser, error) {
	row, exists := s.usersByUsername[strings.TrimSpace(strings.ToLower(username))]
	if !exists {
		return nil, db.ErrNoRows
	}
	copyRow := *row
	return &copyRow, nil
}

func (s *fakeAuthStore) GetUserByID(_ context.Context, userID int64) (*db.AuthUser, error) {
	row, exists := s.usersByID[userID]
	if !exists {
		return nil, db.ErrNoRows
	}
	copyRow := *row
	return &copyRow, nil
}

func (s *fakeAuthStore) CreateSession(_ context.Context, userID int64, expiresAt, now time.Time) (string, error) {
	s.createSessionCalls++
	sessionID := s.createSessionID
	if sessionID == "" {
		sessionID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	}
	s.sessions[sessionID] = &db.AuthSession{
		SessionID:  sessionID,
		UserID:     userID,
		ExpiresAt:  expiresAt,
		LastSeenAt: now,
	}
	return sessionID, nil
}

func (s *fakeAuthStore) SetUserLastLogin(_ context.Context, userID int64, loginAt time.Time) error {
	s.setLastLoginCalls++
	row, exists := s.usersByID[userID]
	if !exists {
		return db.ErrNoRows
	}
	copyTime := loginAt
	row.LastLoginAt = &copyTime
	return nil
}

func (s *fakeAuthStore) DeleteExpiredSessions(_ context.Context, now time.Time) (int64, error) {
	s.deleteExpiredCalls++
	var deleted int64
	for sessionID, row := range s.sessions {
		if !row.ExpiresAt.After(now) {
			delete(s.sessions, sessionID)
			deleted++
		}
	}
	return deleted, nil
}

func (s *fakeAuthStore) EnsureUserSettings(_ context.Context, userID int64) (*db.UserSettingsRecord, error) {
	row, exists := s.settingsByUserID[userID]
	if !exists {
		row = &db.UserSettingsRecord{
			UserID:            userID,
			PreferredLanguage: "en",
			UIPrefs:           json.RawMessage(`{}`),
		}
		s.settingsByUserID[userID] = row
	}

	copyRow := *row
	copyRow.UIPrefs = append(json.RawMessage(nil), row.UIPrefs...)
	return &copyRow, nil
}

func (s *fakeAuthStore) UpsertUserSettings(
	_ context.Context,
	userID int64,
	preferredLanguage string,
	uiPrefs json.RawMessage,
) (*db.UserSettingsRecord, error) {
	copiedUIPrefs := append([]byte(nil), uiPrefs...)
	s.upsertCalls = append(s.upsertCalls, upsertSettingsCall{
		userID:            userID,
		preferredLanguage: preferredLanguage,
		uiPrefs:           copiedUIPrefs,
	})

	row := &db.UserSettingsRecord{
		UserID:            userID,
		PreferredLanguage: preferredLanguage,
		UIPrefs:           append(json.RawMessage(nil), copiedUIPrefs...),
	}
	s.settingsByUserID[userID] = row

	copyRow := *row
	copyRow.UIPrefs = append(json.RawMessage(nil), row.UIPrefs...)
	return &copyRow, nil
}

func (s *fakeAuthStore) SetUserPasswordHash(
	_ context.Context,
	userID int64,
	passwordHash string,
	mustChangePassword bool,
) error {
	s.setPasswordCalls = append(s.setPasswordCalls, setPasswordCall{
		userID:             userID,
		passwordHash:       passwordHash,
		mustChangePassword: mustChangePassword,
	})

	row, exists := s.usersByID[userID]
	if !exists {
		return db.ErrNoRows
	}
	row.PasswordHash = passwordHash
	row.MustChangePassword = mustChangePassword
	return nil
}

func newJSONContext(
	method string,
	path string,
	body string,
) (*echo.Echo, echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e, e.NewContext(req, rec), rec
}

func TestRequireAuth_InvalidSessionCookieReturnsUnauthorizedAndClearsCookie(t *testing.T) {
	t.Parallel()

	store := newFakeAuthStore()
	server := &Server{
		logger:    zerolog.Nop(),
		opts:      Options{SessionCookie: "scoop_session"},
		authStore: store,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.AddCookie(&http.Cookie{Name: "scoop_session", Value: "not-a-valid-uuid"})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := server.requireAuth()(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})
	if err := handler(c); err != nil {
		t.Fatalf("requireAuth returned error: %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusUnauthorized)
	}
	if store.getSessionCalls != 0 {
		t.Fatalf("expected no session lookup for invalid cookie, got %d", store.getSessionCalls)
	}
	if cookie := rec.Header().Get("Set-Cookie"); !strings.Contains(cookie, "scoop_session=") {
		t.Fatalf("expected cleared session cookie, got %q", cookie)
	}
}

func TestHandleLogin_AllowsEmptyPasswordWhenPasswordAuthDisabled(t *testing.T) {
	t.Parallel()

	passwordHash, err := auth.HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	store := newFakeAuthStore()
	store.createSessionID = "11111111-1111-1111-1111-111111111111"
	store.usersByUsername["admin"] = &db.AuthUser{
		UserID:       7,
		Username:     "admin",
		PasswordHash: passwordHash,
		CreatedAt:    time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
	}
	store.usersByID[7] = &db.AuthUser{
		UserID:       7,
		Username:     "admin",
		PasswordHash: passwordHash,
		CreatedAt:    time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
	}
	store.settingsByUserID[7] = &db.UserSettingsRecord{
		UserID:            7,
		PreferredLanguage: "en",
		UIPrefs:           json.RawMessage(`{}`),
	}

	server := &Server{
		logger:    zerolog.Nop(),
		opts:      Options{SessionCookie: "scoop_session", SessionTTL: time.Hour},
		authStore: store,
	}

	_, c, rec := newJSONContext(http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":""}`)
	if err := server.handleLogin(c); err != nil {
		t.Fatalf("handleLogin returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusOK)
	}
	if store.createSessionCalls != 1 {
		t.Fatalf("expected one session creation call, got %d", store.createSessionCalls)
	}
	if store.deleteExpiredCalls != 1 {
		t.Fatalf("expected one expired-session cleanup call, got %d", store.deleteExpiredCalls)
	}
	if cookie := rec.Header().Get("Set-Cookie"); !strings.Contains(cookie, "scoop_session=11111111-1111-1111-1111-111111111111") {
		t.Fatalf("expected session cookie to be set, got %q", cookie)
	}
}

func TestHandleLogin_RejectsInvalidPasswordWhenEnabled(t *testing.T) {
	t.Parallel()

	passwordHash, err := auth.HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	store := newFakeAuthStore()
	store.usersByUsername["admin"] = &db.AuthUser{
		UserID:       7,
		Username:     "admin",
		PasswordHash: passwordHash,
		CreatedAt:    time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
	}
	store.usersByID[7] = &db.AuthUser{
		UserID:       7,
		Username:     "admin",
		PasswordHash: passwordHash,
		CreatedAt:    time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
	}
	store.settingsByUserID[7] = &db.UserSettingsRecord{
		UserID:            7,
		PreferredLanguage: "en",
		UIPrefs:           json.RawMessage(`{"password_enabled":true}`),
	}

	server := &Server{
		logger:    zerolog.Nop(),
		opts:      Options{SessionCookie: "scoop_session", SessionTTL: time.Hour},
		authStore: store,
	}

	_, c, rec := newJSONContext(http.MethodPost, "/api/v1/auth/login", `{"username":"admin","password":"wrong"}`)
	if err := server.handleLogin(c); err != nil {
		t.Fatalf("handleLogin returned error: %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusUnauthorized)
	}
	if store.createSessionCalls != 0 {
		t.Fatalf("did not expect session creation on invalid password, got %d", store.createSessionCalls)
	}
}

func TestHandleLogout_DeletesSessionAndClearsCookie(t *testing.T) {
	t.Parallel()

	store := newFakeAuthStore()
	store.sessions["22222222-2222-2222-2222-222222222222"] = &db.AuthSession{
		SessionID:  "22222222-2222-2222-2222-222222222222",
		UserID:     7,
		Username:   "admin",
		ExpiresAt:  time.Now().UTC().Add(time.Hour),
		LastSeenAt: time.Now().UTC(),
	}

	server := &Server{
		logger:    zerolog.Nop(),
		opts:      Options{SessionCookie: "scoop_session"},
		authStore: store,
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "scoop_session", Value: "22222222-2222-2222-2222-222222222222"})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := server.handleLogout(c); err != nil {
		t.Fatalf("handleLogout returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusOK)
	}
	if len(store.deleteSessionCalls) != 1 || store.deleteSessionCalls[0] != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("unexpected delete session calls: %#v", store.deleteSessionCalls)
	}
	if cookie := rec.Header().Get("Set-Cookie"); !strings.Contains(cookie, "scoop_session=") {
		t.Fatalf("expected cleared session cookie, got %q", cookie)
	}
}

func TestHandlePutMySettings_UpdatesPasswordAndPasswordEnabled(t *testing.T) {
	t.Parallel()

	oldHash, err := auth.HashPassword("old-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	store := newFakeAuthStore()
	store.usersByID[9] = &db.AuthUser{
		UserID:       9,
		Username:     "admin",
		PasswordHash: oldHash,
		CreatedAt:    time.Now().UTC().Add(-time.Hour),
	}
	store.settingsByUserID[9] = &db.UserSettingsRecord{
		UserID:            9,
		PreferredLanguage: "en",
		UIPrefs:           json.RawMessage(`{}`),
	}

	server := &Server{
		logger:    zerolog.Nop(),
		authStore: store,
	}

	_, c, rec := newJSONContext(
		http.MethodPut,
		"/api/v1/me/settings",
		`{"preferred_language":"ZH_cn","password_enabled":true,"password":"new-password"}`,
	)
	c.Set("auth.principal", authPrincipal{UserID: 9, Username: "admin"})

	if err := server.handlePutMySettings(c); err != nil {
		t.Fatalf("handlePutMySettings returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusOK)
	}
	if len(store.setPasswordCalls) != 1 {
		t.Fatalf("expected one password update call, got %d", len(store.setPasswordCalls))
	}
	if !auth.VerifyPassword("new-password", store.setPasswordCalls[0].passwordHash) {
		t.Fatalf("expected stored password hash to match new password")
	}
	if len(store.upsertCalls) != 1 {
		t.Fatalf("expected one settings upsert call, got %d", len(store.upsertCalls))
	}
	if store.upsertCalls[0].preferredLanguage != "zh" {
		t.Fatalf("unexpected preferred language: %q", store.upsertCalls[0].preferredLanguage)
	}

	var uiPrefs map[string]any
	if err := json.Unmarshal(store.upsertCalls[0].uiPrefs, &uiPrefs); err != nil {
		t.Fatalf("decode upsert ui_prefs: %v", err)
	}
	if uiPrefs[passwordEnabledUIPrefKey] != true {
		t.Fatalf("expected password_enabled=true in ui_prefs, got %#v", uiPrefs[passwordEnabledUIPrefKey])
	}
}

func TestHandlePutMySettings_RequiresPasswordWhenEnablingPasswordAuth(t *testing.T) {
	t.Parallel()

	oldHash, err := auth.HashPassword("old-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	store := newFakeAuthStore()
	store.usersByID[11] = &db.AuthUser{
		UserID:       11,
		Username:     "admin",
		PasswordHash: oldHash,
		CreatedAt:    time.Now().UTC().Add(-time.Hour),
	}
	store.settingsByUserID[11] = &db.UserSettingsRecord{
		UserID:            11,
		PreferredLanguage: "en",
		UIPrefs:           json.RawMessage(`{}`),
	}

	server := &Server{
		logger:    zerolog.Nop(),
		authStore: store,
	}

	_, c, rec := newJSONContext(
		http.MethodPut,
		"/api/v1/me/settings",
		`{"password_enabled":true}`,
	)
	c.Set("auth.principal", authPrincipal{UserID: 11, Username: "admin"})

	if err := server.handlePutMySettings(c); err != nil {
		t.Fatalf("handlePutMySettings returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d", rec.Code, http.StatusBadRequest)
	}
	if len(store.setPasswordCalls) != 0 {
		t.Fatalf("did not expect password update calls, got %d", len(store.setPasswordCalls))
	}
}
