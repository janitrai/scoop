package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"horse.fit/scoop/internal/language"
)

const defaultPreferredLanguage = "en"

type AuthUser struct {
	UserID             int64      `json:"user_id"`
	Username           string     `json:"username"`
	PasswordHash       string     `json:"-"`
	MustChangePassword bool       `json:"must_change_password"`
	CreatedAt          time.Time  `json:"created_at"`
	LastLoginAt        *time.Time `json:"last_login_at,omitempty"`
}

type AuthSession struct {
	SessionID          string    `json:"session_id"`
	UserID             int64     `json:"user_id"`
	Username           string    `json:"username"`
	MustChangePassword bool      `json:"must_change_password"`
	ExpiresAt          time.Time `json:"expires_at"`
	LastSeenAt         time.Time `json:"last_seen_at"`
}

type UserSettingsRecord struct {
	UserID            int64           `json:"user_id"`
	PreferredLanguage string          `json:"preferred_language"`
	UIPrefs           json.RawMessage `json:"ui_prefs"`
}

func (p *Pool) CountUsers(ctx context.Context) (int64, error) {
	const q = `SELECT COUNT(*) FROM news.users`

	var count int64
	if err := p.QueryRow(ctx, q).Scan(&count); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return count, nil
}

func (p *Pool) CreateUser(ctx context.Context, username, passwordHash string, mustChangePassword bool) (*AuthUser, error) {
	const q = `
INSERT INTO news.users (
	username,
	password_hash,
	must_change_password,
	created_at
)
VALUES ($1, $2, $3, now())
RETURNING
	user_id,
	username,
	password_hash,
	must_change_password,
	created_at,
	last_login_at
`

	var row AuthUser
	if err := p.QueryRow(ctx, q, normalizeUsername(username), strings.TrimSpace(passwordHash), mustChangePassword).Scan(
		&row.UserID,
		&row.Username,
		&row.PasswordHash,
		&row.MustChangePassword,
		&row.CreatedAt,
		&row.LastLoginAt,
	); err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	return &row, nil
}

func (p *Pool) GetUserByUsername(ctx context.Context, username string) (*AuthUser, error) {
	const q = `
SELECT
	user_id,
	username,
	password_hash,
	must_change_password,
	created_at,
	last_login_at
FROM news.users
WHERE username = $1
LIMIT 1
`

	var row AuthUser
	if err := p.QueryRow(ctx, q, normalizeUsername(username)).Scan(
		&row.UserID,
		&row.Username,
		&row.PasswordHash,
		&row.MustChangePassword,
		&row.CreatedAt,
		&row.LastLoginAt,
	); err != nil {
		if IsNoRows(err) {
			return nil, ErrNoRows
		}
		return nil, fmt.Errorf("query user by username: %w", err)
	}
	return &row, nil
}

func (p *Pool) GetUserByID(ctx context.Context, userID int64) (*AuthUser, error) {
	const q = `
SELECT
	user_id,
	username,
	password_hash,
	must_change_password,
	created_at,
	last_login_at
FROM news.users
WHERE user_id = $1
LIMIT 1
`

	var row AuthUser
	if err := p.QueryRow(ctx, q, userID).Scan(
		&row.UserID,
		&row.Username,
		&row.PasswordHash,
		&row.MustChangePassword,
		&row.CreatedAt,
		&row.LastLoginAt,
	); err != nil {
		if IsNoRows(err) {
			return nil, ErrNoRows
		}
		return nil, fmt.Errorf("query user by id: %w", err)
	}
	return &row, nil
}

func (p *Pool) SetUserLastLogin(ctx context.Context, userID int64, loginAt time.Time) error {
	const q = `
UPDATE news.users
SET last_login_at = $2
WHERE user_id = $1
`

	tag, err := p.Exec(ctx, q, userID, loginAt.UTC())
	if err != nil {
		return fmt.Errorf("update user last login: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoRows
	}
	return nil
}

func (p *Pool) CreateSession(ctx context.Context, userID int64, expiresAt, now time.Time) (string, error) {
	const q = `
INSERT INTO news.sessions (
	user_id,
	expires_at,
	created_at,
	last_seen_at
)
VALUES ($1, $2, $3, $3)
RETURNING session_id::text
`

	var sessionID string
	if err := p.QueryRow(ctx, q, userID, expiresAt.UTC(), now.UTC()).Scan(&sessionID); err != nil {
		return "", fmt.Errorf("insert session: %w", err)
	}
	return sessionID, nil
}

func (p *Pool) GetSession(ctx context.Context, sessionID string) (*AuthSession, error) {
	const q = `
SELECT
	s.session_id::text,
	s.user_id,
	u.username,
	u.must_change_password,
	s.expires_at,
	s.last_seen_at
FROM news.sessions s
JOIN news.users u
	ON u.user_id = s.user_id
WHERE s.session_id = $1::uuid
LIMIT 1
`

	var row AuthSession
	if err := p.QueryRow(ctx, q, strings.TrimSpace(sessionID)).Scan(
		&row.SessionID,
		&row.UserID,
		&row.Username,
		&row.MustChangePassword,
		&row.ExpiresAt,
		&row.LastSeenAt,
	); err != nil {
		if IsNoRows(err) {
			return nil, ErrNoRows
		}
		return nil, fmt.Errorf("query session: %w", err)
	}
	return &row, nil
}

func (p *Pool) TouchSession(ctx context.Context, sessionID string, seenAt time.Time) error {
	const q = `
UPDATE news.sessions
SET last_seen_at = $2
WHERE session_id = $1::uuid
`

	tag, err := p.Exec(ctx, q, strings.TrimSpace(sessionID), seenAt.UTC())
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoRows
	}
	return nil
}

func (p *Pool) DeleteSession(ctx context.Context, sessionID string) error {
	const q = `
DELETE FROM news.sessions
WHERE session_id = $1::uuid
`

	if _, err := p.Exec(ctx, q, strings.TrimSpace(sessionID)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (p *Pool) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	const q = `
DELETE FROM news.sessions
WHERE expires_at <= $1
`

	tag, err := p.Exec(ctx, q, now.UTC())
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (p *Pool) EnsureUserSettings(ctx context.Context, userID int64) (*UserSettingsRecord, error) {
	const ensureQ = `
INSERT INTO news.user_settings (user_id, preferred_language, ui_prefs)
VALUES ($1, $2, '{}'::jsonb)
ON CONFLICT (user_id) DO NOTHING
`

	if _, err := p.Exec(ctx, ensureQ, userID, defaultPreferredLanguage); err != nil {
		return nil, fmt.Errorf("ensure user settings row: %w", err)
	}

	return p.GetUserSettings(ctx, userID)
}

func (p *Pool) GetUserSettings(ctx context.Context, userID int64) (*UserSettingsRecord, error) {
	const q = `
SELECT
	user_id,
	preferred_language,
	ui_prefs
FROM news.user_settings
WHERE user_id = $1
LIMIT 1
`

	var (
		row     UserSettingsRecord
		uiPrefs []byte
	)
	if err := p.QueryRow(ctx, q, userID).Scan(
		&row.UserID,
		&row.PreferredLanguage,
		&uiPrefs,
	); err != nil {
		if IsNoRows(err) {
			return nil, ErrNoRows
		}
		return nil, fmt.Errorf("query user settings: %w", err)
	}

	row.PreferredLanguage = strings.TrimSpace(row.PreferredLanguage)
	if row.PreferredLanguage == "" {
		row.PreferredLanguage = defaultPreferredLanguage
	}
	if len(uiPrefs) == 0 {
		uiPrefs = []byte("{}")
	}
	row.UIPrefs = append(json.RawMessage(nil), uiPrefs...)
	return &row, nil
}

func (p *Pool) UpsertUserSettings(
	ctx context.Context,
	userID int64,
	preferredLanguage string,
	uiPrefs json.RawMessage,
) (*UserSettingsRecord, error) {
	if len(uiPrefs) == 0 {
		uiPrefs = json.RawMessage(`{}`)
	}

	const q = `
INSERT INTO news.user_settings (
	user_id,
	preferred_language,
	ui_prefs
)
VALUES ($1, $2, $3::jsonb)
ON CONFLICT (user_id)
DO UPDATE SET
	preferred_language = EXCLUDED.preferred_language,
	ui_prefs = EXCLUDED.ui_prefs
RETURNING
	user_id,
	preferred_language,
	ui_prefs
`

	var (
		row          UserSettingsRecord
		storedUIPref []byte
	)
	if err := p.QueryRow(
		ctx,
		q,
		userID,
		normalizePreferredLanguage(preferredLanguage),
		uiPrefs,
	).Scan(
		&row.UserID,
		&row.PreferredLanguage,
		&storedUIPref,
	); err != nil {
		return nil, fmt.Errorf("upsert user settings: %w", err)
	}

	if len(storedUIPref) == 0 {
		storedUIPref = []byte("{}")
	}
	row.UIPrefs = append(json.RawMessage(nil), storedUIPref...)
	return &row, nil
}

func (p *Pool) SetUserPasswordHash(
	ctx context.Context,
	userID int64,
	passwordHash string,
	mustChangePassword bool,
) error {
	const q = `
UPDATE news.users
SET
	password_hash = $2,
	must_change_password = $3
WHERE user_id = $1
`

	tag, err := p.Exec(ctx, q, userID, strings.TrimSpace(passwordHash), mustChangePassword)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoRows
	}
	return nil
}

func normalizeUsername(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizePreferredLanguage(raw string) string {
	lang := language.NormalizeTag(raw)
	if lang == "" {
		return defaultPreferredLanguage
	}
	return lang
}
