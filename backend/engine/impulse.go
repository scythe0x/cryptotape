package engine

import (
	"math"
	"time"
)

type impulse struct {
	ID         string
	Direction  int
	StartPrice float64
	StartAt    time.Time
	LastPrice  float64
	LastAt     time.Time
	MaxPtc     float64
}

func sign(pct float64) int {
	if pct >= 0 {
		return 1
	}
	return -1
}

// detect impulse based on extrema
func detectMove(p pricePoint, min pricePoint, max pricePoint, threshold float64) (bool, float64, pricePoint) {
	upMove := (p.Price - min.Price) / min.Price * 100
	if upMove >= threshold {
		return true, upMove, min
	}

	downMove := (p.Price - max.Price) / max.Price * 100
	if downMove <= -threshold {
		return true, downMove, max
	}

	return false, 0, pricePoint{}
}

func shouldContinueImpulse(imp *impulse, p pricePoint, dir int, maxGap time.Duration) bool {
	if imp == nil || imp.Direction != dir {
		return false
	}
	if p.At.Sub(imp.LastAt) > maxGap {
		return false
	}

	progressFromStart := float64(dir) * (p.Price - imp.StartPrice)
	return progressFromStart > 0 || math.Abs(progressFromStart) < 1e-12
}
