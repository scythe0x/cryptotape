package binance

import (
	"sort"
	"strings"
	"time"

	"cryptotape/internal/core"
)

func (f *Feed) status(state, detail string) {
	now := time.Now()
	f.mu.Lock()
	health := core.Health{State: state, Detail: detail, Symbols: len(f.symbols), UpdatedAtMs: now.UnixMilli(), UsedWeight: f.usedWeight, Streams: []core.StreamHealth{}}
	if !f.volumeUpdated.IsZero() {
		health.VolumeAgeMs = now.Sub(f.volumeUpdated).Milliseconds()
	}
	var serviceErrors []string
	for name, err := range f.serviceErrors {
		serviceErrors = append(serviceErrors, name+": "+err)
	}
	sort.Strings(serviceErrors)
	health.ServiceError = strings.Join(serviceErrors, "; ")
	for shard := range f.shardMembers {
		stream := f.streams[shard]
		item := core.StreamHealth{Shard: shard, State: "connecting"}
		if stream != nil {
			item = stream.health
			item.AgeP99Ms, item.AgeMaxMs = stream.histogram.snapshot(now)
			if !stream.received.IsZero() && item.Subscriptions > 0 {
				age := max(int64(0), (stream.age + now.Sub(stream.received)).Milliseconds())
				item.QuoteAgeMs = &age
				if health.QuoteAgeMs == nil || age > *health.QuoteAgeMs {
					health.QuoteAgeMs = &age
				}
			}
			if item.State == "connected" || item.State == "idle" {
				health.Connections++
			}
		}
		if state == "warming" && item.State != "connected" && item.State != "idle" {
			if item.State == "error" {
				health.State = "degraded"
			} else if health.State != "degraded" {
				health.State = "reconnecting"
			}
		}
		health.Streams = append(health.Streams, item)
	}
	f.mu.Unlock()
	if f.cb.Health != nil {
		f.cb.Health(health)
	}
}
