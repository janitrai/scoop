package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("changeme123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if hash == "" {
		t.Fatalf("expected non-empty hash")
	}
	if !VerifyPassword("changeme123", hash) {
		t.Fatalf("expected password verification to succeed")
	}
	if VerifyPassword("wrong-password", hash) {
		t.Fatalf("did not expect wrong password to verify")
	}
}

func TestNormalizeUsername(t *testing.T) {
	t.Parallel()

	if got := NormalizeUsername(" Admin "); got != "admin" {
		t.Fatalf("unexpected normalized username: %q", got)
	}
}
