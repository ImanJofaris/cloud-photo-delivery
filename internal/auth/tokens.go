package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const tokenIssuer = "cloud-photo-delivery"

var ErrInvalidToken = errors.New("invalid token")

type TokenService struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

func NewTokenService(secret string, accessTTL, refreshTTL time.Duration) *TokenService {
	return &TokenService{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		now:        time.Now,
	}
}

func (t *TokenService) AccessTTL() time.Duration { return t.accessTTL }

// IssueAccessToken returns a signed HS256 JWT for the given user.
func (t *TokenService) IssueAccessToken(userID uuid.UUID) (string, error) {
	now := t.now()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		Issuer:    tokenIssuer,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(t.accessTTL)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(t.secret)
}

func (t *TokenService) VerifyAccessToken(tokenString string) (uuid.UUID, error) {
	parsed, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{},
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return t.secret, nil
		},
		jwt.WithIssuer(tokenIssuer),
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil || !parsed.Valid {
		return uuid.Nil, ErrInvalidToken
	}
	claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.Subject == "" {
		return uuid.Nil, ErrInvalidToken
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	return id, nil
}

// NewOpaqueToken returns a random URL-safe token and its SHA-256 hash.
func NewOpaqueToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, HashToken(raw), nil
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func TokensEqual(hash, raw string) bool {
	return subtle.ConstantTimeCompare([]byte(hash), []byte(HashToken(raw))) == 1
}
