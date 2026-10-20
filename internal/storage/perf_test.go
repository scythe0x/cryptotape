package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"cryptotape/internal/core"
)

func BenchmarkPersistAlertBatch(b *testing.B) {
	for _, size := range []int{1, 64} {
		b.Run(fmt.Sprintf("%d_alerts", size), func(b *testing.B) {
			store, err := Open(filepath.Join(b.TempDir(), "perf.sqlite"))
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() {
				if err := store.Close(); err != nil {
					b.Error(err)
				}
			})
			now := time.Now()
			candidates := make([]core.Candidate, size)
			for i := range candidates {
				candidates[i] = core.Candidate{Rule: core.Rule{ID: 1, Sound: "none"}, Symbol: fmt.Sprintf("COIN%dUSDT", i), Received: now, Detected: now, EventTime: now, Price: 90, ChangePercent: -10}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				alerts, err := store.InsertAlerts(context.Background(), candidates)
				if err != nil || len(alerts) != size {
					b.Fatalf("inserted %d/%d alerts: %v", len(alerts), size, err)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(b.N)*float64(size)/b.Elapsed().Seconds(), "alerts/s")
		})
	}
}
