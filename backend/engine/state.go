package main

import "time"

// symbolState holds sliding window and extrema
type symbolState struct {
	points []pricePoint
	min    pricePoint
	max    pricePoint
}

func newSymbolState() *symbolState {
	return &symbolState{
		points: make([]pricePoint, 0, 128),
	}
}

func (s *symbolState) addPoint(p pricePoint) {
	s.points = append(s.points, p)

	if len(s.points) == 1 {
		s.min = p
		s.max = p
		return
	}

	if p.Price < s.min.Price {
		s.min = p
	}
	if p.Price > s.max.Price {
		s.max = p
	}
}

func (s *symbolState) trim(now time.Time, maxWindow time.Duration) {
	cutoff := now.Add(-maxWindow)

	i := 0
	for i < len(s.points) && s.points[i].At.Before(cutoff) {
		i++
	}

	if i > 0 {
		s.points = s.points[i:]
	}

	if len(s.points) == 0 {
		return
	}

	// recalc extrema if needed
	if s.min.At.Before(cutoff) || s.max.At.Before(cutoff) {
		min := s.points[0]
		max := s.points[0]

		for _, p := range s.points {
			if p.Price < min.Price {
				min = p
			}
			if p.Price > max.Price {
				max = p
			}
		}

		s.min = min
		s.max = max
	}
}