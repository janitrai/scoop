package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"horse.fit/scoop/internal/auth"
	"horse.fit/scoop/internal/config"
	"horse.fit/scoop/internal/db"
)

func ensureDefaultAdmin(ctx context.Context, pool *db.Pool, cfg *config.Config, logger zerolog.Logger) error {
	if pool == nil || cfg == nil {
		return fmt.Errorf("ensure default admin: missing dependencies")
	}

	userCount, err := pool.CountUsers(ctx)
	if err != nil {
		return err
	}
	if userCount > 0 {
		return nil
	}

	username := auth.NormalizeUsername(cfg.DefaultAdminUser)
	password := strings.TrimSpace(cfg.DefaultAdminPassword)
	if username == "" {
		return fmt.Errorf("default admin username is empty")
	}

	passwordHash, err := hashDefaultAdminPassword(password)
	if err != nil {
		return fmt.Errorf("hash default admin password: %w", err)
	}

	user, err := pool.CreateUser(ctx, username, passwordHash, cfg.DefaultAdminMustChangePassword)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key value") {
			return nil
		}
		return err
	}

	if _, err := pool.EnsureUserSettings(ctx, user.UserID); err != nil {
		return err
	}

	logger.Warn().
		Str("username", username).
		Bool("must_change_password", cfg.DefaultAdminMustChangePassword).
		Msg("created default admin user")

	return nil
}

func hashDefaultAdminPassword(password string) (string, error) {
	trimmed := strings.TrimSpace(password)
	if trimmed != "" {
		return auth.HashPassword(trimmed)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(""), auth.DefaultBcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
