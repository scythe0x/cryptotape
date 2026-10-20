package core

import "time"

const bucketSize = 100 * time.Millisecond

type extremum struct {
	bucket int64
	value  float64
}

type window struct {
	duration int64
	started  time.Time
	minima   []extremum
	maxima   []extremum
	minHead  int
	maxHead  int
}

func newWindow(duration time.Duration) *window {
	return &window{duration: int64((duration + bucketSize - 1) / bucketSize)}
}

func (w *window) add(at time.Time, value float64) {
	if w.started.IsZero() {
		w.started = at
	}
	bucket := at.UnixNano() / int64(bucketSize)
	w.minima, w.minHead = expire(w.minima, w.minHead, bucket-w.duration)
	w.maxima, w.maxHead = expire(w.maxima, w.maxHead, bucket-w.duration)

	if len(w.minima) == w.minHead || w.minima[len(w.minima)-1].bucket != bucket || value < w.minima[len(w.minima)-1].value {
		for len(w.minima) > w.minHead && w.minima[len(w.minima)-1].value >= value {
			w.minima = w.minima[:len(w.minima)-1]
		}
		w.minima = append(w.minima, extremum{bucket, value})
	}

	if len(w.maxima) == w.maxHead || w.maxima[len(w.maxima)-1].bucket != bucket || value > w.maxima[len(w.maxima)-1].value {
		for len(w.maxima) > w.maxHead && w.maxima[len(w.maxima)-1].value <= value {
			w.maxima = w.maxima[:len(w.maxima)-1]
		}
		w.maxima = append(w.maxima, extremum{bucket, value})
	}
}

func (w *window) bounds() (float64, float64) {
	return w.minima[w.minHead].value, w.maxima[w.maxHead].value
}

func expire(items []extremum, head int, cutoff int64) ([]extremum, int) {
	for head < len(items) && items[head].bucket < cutoff {
		head++
	}
	if head > 1024 && head*2 > len(items) {
		items = append([]extremum(nil), items[head:]...)
		head = 0
	}
	return items, head
}
