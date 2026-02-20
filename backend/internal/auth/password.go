package auth

import (
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const DefaultBcryptCost = 12

func HashPassword(password string) (string, error) {
	trimmed := strings.TrimSpace(password)
	if trimmed == "" {
		return "", fmt.Errorf("password is required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(trimmed), DefaultBcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func VerifyPassword(password, hash string) bool {
	trimmedPassword := strings.TrimSpace(password)
	trimmedHash := strings.TrimSpace(hash)
	if trimmedPassword == "" || trimmedHash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(trimmedHash), []byte(trimmedPassword)) == nil
}

func NormalizeUsername(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}
