package main

import "time"

type impulse struct {
	ID         string
	Direction  int
	StartPrice float64
	StartAt    time.Time
	LastPrice  float64
	LastAt     time.Time
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

// check if we continue same impulse
func isSameImpulse(imp *impulse, base pricePoint, dir int) bool {
	return imp != nil &&
		base.At.Before(imp.LastAt) &&
		imp.Direction == dir
}