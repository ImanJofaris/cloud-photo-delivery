package devices

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

const (
	KeyScheme    = "cpd_live_"
	prefixLength = 12
	prefixBytes  = 6
	secretBytes  = 32
)

var ErrMalformedKey = errors.New("malformed device key")

func GenerateKey() (raw, prefix, hash string, err error) {
	prefixRaw := make([]byte, prefixBytes)
	if _, err := rand.Read(prefixRaw); err != nil {
		return "", "", "", err
	}
	secret := make([]byte, secretBytes)
	if _, err := rand.Read(secret); err != nil {
		return "", "", "", err
	}
	prefix = hex.EncodeToString(prefixRaw)
	raw = KeyScheme + prefix + "_" + base64.RawURLEncoding.EncodeToString(secret)
	return raw, prefix, HashKey(raw), nil
}

func HashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func KeyPrefix(raw string) (string, error) {
	rest, ok := strings.CutPrefix(raw, KeyScheme)
	if !ok {
		return "", ErrMalformedKey
	}
	i := strings.IndexByte(rest, '_')
	if i != prefixLength {
		return "", ErrMalformedKey
	}
	prefix := rest[:i]
	decoded, err := hex.DecodeString(prefix)
	if err != nil || len(decoded) != prefixBytes {
		return "", ErrMalformedKey
	}
	if len(rest) <= i+1 {
		return "", ErrMalformedKey
	}
	return prefix, nil
}

func VerifyKey(raw, hash string) bool {
	return subtle.ConstantTimeCompare([]byte(HashKey(raw)), []byte(hash)) == 1
}
