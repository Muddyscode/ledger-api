package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWTRoundTrip(t *testing.T) {
	secret := "dev-only-change-me-use-32chars-min!!"
	id := uuid.Must(uuid.NewV7())
	tok, exp, err := IssueToken(secret, "op@example.com", id, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(exp) < 50*time.Minute {
		t.Fatal("expiry too soon")
	}
	got, email, err := ParseToken(secret, tok)
	if err != nil {
		t.Fatal(err)
	}
	if got != id || email != "op@example.com" {
		t.Fatalf("got %s %s", got, email)
	}
	if _, _, err := ParseToken("other-secret-which-is-32-bytes!!", tok); err == nil {
		t.Fatal("wrong secret accepted")
	}
}
