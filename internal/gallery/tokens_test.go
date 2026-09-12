package gallery

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUnlockTokens_IssueAndVerify(t *testing.T) {
	svc := NewUnlockTokens("secret", 30*time.Minute)
	eventID := uuid.New()

	token, err := svc.Issue(eventID)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := svc.Verify(token, eventID, nil)
	require.NoError(t, err)
	require.Equal(t, eventID.String(), claims.EventID)
}

func TestUnlockTokens_WrongEventRejected(t *testing.T) {
	svc := NewUnlockTokens("secret", 30*time.Minute)
	token, err := svc.Issue(uuid.New())
	require.NoError(t, err)

	_, err = svc.Verify(token, uuid.New(), nil)
	require.ErrorIs(t, err, ErrInvalidUnlockToken)
}

func TestUnlockTokens_WrongSecretRejected(t *testing.T) {
	issuer := NewUnlockTokens("secret-a", time.Minute)
	verifier := NewUnlockTokens("secret-b", time.Minute)
	token, err := issuer.Issue(uuid.New())
	require.NoError(t, err)

	_, err = verifier.Verify(token, uuid.New(), nil)
	require.ErrorIs(t, err, ErrInvalidUnlockToken)
}

func TestUnlockTokens_ExpiredRejected(t *testing.T) {
	svc := NewUnlockTokens("secret", time.Minute)
	past := time.Now().Add(-2 * time.Minute)
	svc.SetClock(func() time.Time { return past })
	eventID := uuid.New()
	token, err := svc.Issue(eventID)
	require.NoError(t, err)

	svc.SetClock(time.Now)
	_, err = svc.Verify(token, eventID, nil)
	require.ErrorIs(t, err, ErrInvalidUnlockToken)
}

func TestUnlockTokens_RejectsAccessTokenIssuer(t *testing.T) {
	// A token signed with the same secret but a different issuer must not
	// verify as an unlock token.
	access := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "cloud-photo-delivery",
		Subject:   uuid.NewString(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	raw, err := access.SignedString([]byte("secret"))
	require.NoError(t, err)

	svc := NewUnlockTokens("secret", time.Minute)
	_, err = svc.Verify(raw, uuid.New(), nil)
	require.ErrorIs(t, err, ErrInvalidUnlockToken)
}

func TestUnlockTokens_NotBeforeRevokesOlderTokens(t *testing.T) {
	svc := NewUnlockTokens("secret", time.Hour)
	issued := time.Now().Add(-10 * time.Minute)
	svc.SetClock(func() time.Time { return issued })
	eventID := uuid.New()
	token, err := svc.Issue(eventID)
	require.NoError(t, err)

	changed := issued.Add(time.Minute)
	_, err = svc.Verify(token, eventID, &changed)
	require.ErrorIs(t, err, ErrInvalidUnlockToken)

	_, err = svc.Verify(token, eventID, nil)
	require.NoError(t, err)
}
