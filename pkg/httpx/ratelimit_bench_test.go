package httpx

import (
	"fmt"
	"testing"
	"time"
)

// Temporary diagnostic benchmark; deleted after measurement.
func BenchmarkRateLimiter_ChargeWithKeySpace(b *testing.B) {
	for _, keys := range []int{1000, 10000, 100000} {
		b.Run(fmt.Sprintf("keys=%d", keys), func(b *testing.B) {
			rl := NewRateLimiter(600, 300, 10*time.Minute)
			base := time.Now()
			rl.now = func() time.Time { return base }
			for i := 0; i < keys; i++ {
				rl.Allow(fmt.Sprintf("10.0.%d.%d|event", i/250, i%250))
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				rl.Allow("hot-key")
			}
		})
	}
}
