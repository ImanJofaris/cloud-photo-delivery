package billing

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePlanLimits(t *testing.T) {
	t.Run("decodes every limit", func(t *testing.T) {
		raw := []byte(`{
			"events": 5,
			"photosPerEvent": 5000,
			"storageBytes": 53687091200,
			"retentionDays": 30,
			"apiAccess": true,
			"branding": true,
			"originalDownloads": true
		}`)
		limits, err := parsePlanLimits(raw)
		require.NoError(t, err)
		require.Equal(t, PlanLimits{
			Events:            5,
			PhotosPerEvent:    5000,
			StorageBytes:      53687091200,
			RetentionDays:     30,
			APIAccess:         true,
			Branding:          true,
			OriginalDownloads: true,
		}, limits)
	})

	t.Run("zero events stays unlimited", func(t *testing.T) {
		limits, err := parsePlanLimits([]byte(`{"events": 0, "retentionDays": 90}`))
		require.NoError(t, err)
		require.Equal(t, 0, limits.Events)
	})

	t.Run("missing events fails closed", func(t *testing.T) {
		_, err := parsePlanLimits([]byte(`{"activeEvents": 1, "retentionDays": 7}`))
		require.Error(t, err)
	})

	t.Run("invalid JSON fails", func(t *testing.T) {
		_, err := parsePlanLimits([]byte(`not json`))
		require.Error(t, err)
	})
}
