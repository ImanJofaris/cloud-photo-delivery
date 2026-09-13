package devices

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateKey_Format(t *testing.T) {
	raw, prefix, hash, err := GenerateKey()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(raw, KeyScheme), "raw key scheme")
	require.Len(t, prefix, prefixLength)
	require.Contains(t, raw, "_"+prefix+"_")
	require.Equal(t, HashKey(raw), hash)

	parsed, err := KeyPrefix(raw)
	require.NoError(t, err)
	require.Equal(t, prefix, parsed)
	require.True(t, VerifyKey(raw, hash))
}

func TestGenerateKey_Unique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		raw, prefix, _, err := GenerateKey()
		require.NoError(t, err)
		require.False(t, seen[raw], "raw key reused")
		require.False(t, seen[prefix], "prefix reused")
		seen[raw] = true
		seen[prefix] = true
	}
}

func TestKeyPrefix_RejectsMalformed(t *testing.T) {
	_, _, hash, err := GenerateKey()
	require.NoError(t, err)

	cases := []string{
		"",
		"cpd_live_",
		"cpd_live_short_secret",
		"cpd_live_0123456789ab",
		"cpd_live_0123456789ab_",
		"cpd_live_zzzzzzzzzzzz_secret",
		"invalid_0123456789ab_secret",
		"cpd_live_0123456789ab_secret_extra",
	}
	for _, c := range cases {
		_, err := KeyPrefix(c)
		if c == "cpd_live_0123456789ab_secret_extra" {
			require.NoError(t, err, "suffix after secret is part of the secret")
			continue
		}
		require.ErrorIs(t, err, ErrMalformedKey, "key=%q", c)
	}

	require.False(t, VerifyKey("cpd_live_wrong_secret", hash))
	require.False(t, VerifyKey("", hash))
}

func TestHashKey_Deterministic(t *testing.T) {
	raw := "cpd_live_0123456789ab_secretvalue"
	require.Equal(t, HashKey(raw), HashKey(raw))
	require.NotEqual(t, HashKey(raw), HashKey(raw+"x"))
}
