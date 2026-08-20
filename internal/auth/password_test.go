package auth

import "testing"

func TestPasswordHashRoundTrip(t *testing.T) {
	h, err := HashPassword("change-me-now-12")
	if err != nil {
		t.Fatal(err)
	}
	if h == "change-me-now-12" {
		t.Fatal("password stored in plaintext")
	}
	if !VerifyPassword("change-me-now-12", h) {
		t.Fatal("correct password rejected")
	}
	if VerifyPassword("wrong-password-xx", h) {
		t.Fatal("wrong password accepted")
	}
}
