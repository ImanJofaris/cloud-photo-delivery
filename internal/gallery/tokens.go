package gallery

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// unlockIssuer is deliberately distinct from the account token issuer so an
// access token can never be replayed as a gallery unlock token (or vice versa).
const unlockIssuer = "cloud-photo-delivery-gallery"

var ErrInvalidUnlockToken = errors.New("invalid unlock token")

// UnlockClaims are the claims carried by an event-scoped unlock token.
type UnlockClaims struct {
	EventID string `json:"event_id"`
	jwt.RegisteredClaims
}

// UnlockTokens issues and verifies event-scoped gallery unlock tokens.
type UnlockTokens struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewUnlockTokens(secret string, ttl time.Duration) *UnlockTokens {
	return &UnlockTokens{secret: []byte(secret), ttl: ttl, now: time.Now}
}

func (t *UnlockTokens) TTL() time.Duration { return t.ttl }

func (t *UnlockTokens) SetClock(now func() time.Time) {
	if now != nil {
		t.now = now
	}
}

// Issue returns a signed unlock token scoped to eventID.
func (t *UnlockTokens) Issue(eventID uuid.UUID) (string, error) {
	now := t.now()
	claims := UnlockClaims{
		EventID: eventID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    unlockIssuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(t.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(t.secret)
}

// Verify returns the event ID from a valid unlock token. It rejects tokens
// issued before notBefore (used for password-change revocation) and tokens
// scoped to a different event than expected.
func (t *UnlockTokens) Verify(tokenString string, expected uuid.UUID, notBefore *time.Time) (*UnlockClaims, error) {
	parsed, err := jwt.ParseWithClaims(tokenString, &UnlockClaims{},
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return t.secret, nil
		},
		jwt.WithIssuer(unlockIssuer),
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil || !parsed.Valid {
		return nil, ErrInvalidUnlockToken
	}
	claims, ok := parsed.Claims.(*UnlockClaims)
	if !ok || claims.EventID == "" {
		return nil, ErrInvalidUnlockToken
	}
	got, err := uuid.Parse(claims.EventID)
	if err != nil || got != expected {
		return nil, ErrInvalidUnlockToken
	}
	if notBefore != nil && claims.IssuedAt != nil && claims.IssuedAt.Before(*notBefore) {
		return nil, ErrInvalidUnlockToken
	}
	return claims, nil
}
