package binance

import "time"

var ageBounds = [...]int64{0, 5, 10, 25, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 30000}

type ageBucket struct {
	second int64
	bins   [len(ageBounds) + 1]uint64
	max    int64
}

type ageHistogram [30]ageBucket

func (h *ageHistogram) add(at time.Time, age time.Duration) {
	second := at.Unix()
	slot := &h[second%int64(len(h))]
	if slot.second != second {
		*slot = ageBucket{second: second}
	}
	ms := max(int64(0), age.Milliseconds())
	index := len(ageBounds)
	for i, bound := range ageBounds {
		if ms <= bound {
			index = i
			break
		}
	}
	slot.bins[index]++
	slot.max = max(slot.max, ms)
}

// p99 is the upper bound of its histogram bin over the last 30 seconds.
func (h *ageHistogram) snapshot(now time.Time) (p99, maximum int64) {
	var bins [len(ageBounds) + 1]uint64
	var count uint64
	for _, slot := range h {
		if slot.second <= now.Unix()-30 || slot.second > now.Unix() {
			continue
		}
		maximum = max(maximum, slot.max)
		for i, n := range slot.bins {
			bins[i] += n
			count += n
		}
	}
	target := (count*99 + 99) / 100
	var seen uint64
	for i, n := range bins {
		seen += n
		if count > 0 && seen >= target {
			if i == len(ageBounds) {
				return maximum, maximum
			}
			return ageBounds[i], maximum
		}
	}
	return 0, maximum
}
