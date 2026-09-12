package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestPasswordHashVerify(t *testing.T) {
	hash, err := HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "password123" {
		t.Fatal("password must not be stored in plaintext")
	}
	if !VerifyPassword(hash, "password123") {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword(hash, "wrong") {
		t.Fatal("wrong password must not verify")
	}
}

func TestAccessToken_IssueVerify(t *testing.T) {
	ts := NewTokenService("secret", 15*time.Minute, time.Hour)
	id := uuid.New()
	token, err := ts.IssueAccessToken(id)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ts.VerifyAccessToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatalf("got %s want %s", got, id)
	}
}

func TestAccessToken_Tampered(t *testing.T) {
	ts := NewTokenService("secret", 15*time.Minute, time.Hour)
	token, _ := ts.IssueAccessToken(uuid.New())

	other := NewTokenService("different-secret", 15*time.Minute, time.Hour)
	if _, err := other.VerifyAccessToken(token); err == nil {
		t.Fatal("expected verification to fail with different secret")
	}
}

func TestAccessToken_Expired(t *testing.T) {
	ts := NewTokenService("secret", -time.Minute, time.Hour)
	token, _ := ts.IssueAccessToken(uuid.New())
	if _, err := ts.VerifyAccessToken(token); err == nil {
		t.Fatal("expected expired token to fail")
	}
}

func TestAccessToken_RejectsNoneAlg(t *testing.T) {
	ts := NewTokenService("secret", 15*time.Minute, time.Hour)
	claims := jwt.RegisteredClaims{Subject: uuid.New().String(), Issuer: tokenIssuer}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts.VerifyAccessToken(signed); err == nil {
		t.Fatal("expected 'none' algorithm to be rejected")
	}
}

func TestOpaqueToken(t *testing.T) {
	raw, hash, err := NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" || hash == "" {
		t.Fatal("expected non-empty token and hash")
	}
	if raw == hash {
		t.Fatal("hash must differ from raw token")
	}
	if !TokensEqual(hash, raw) {
		t.Fatal("expected TokensEqual to match")
	}
	if TokensEqual(hash, "something-else") {
		t.Fatal("expected mismatch")
	}
	if HashToken(raw) != hash {
		t.Fatal("hash must be deterministic")
	}
}
