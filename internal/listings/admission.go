package listings

import (
	"context"
	"crypto/sha256"
	"sync"
	"time"
)

var admissions = struct {
	sync.Mutex
	until map[[32]byte]time.Time
}{until: make(map[[32]byte]time.Time)}

func (c *Client) waitAdmission(ctx context.Context) error {
	key := sha256.Sum256([]byte(c.endpoint + "\x00" + c.key))
	for {
		admissions.Lock()
		delay := time.Until(admissions.until[key])
		admissions.Unlock()
		if delay <= 0 {
			return nil
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return context.Cause(ctx)
		case <-timer.C:
		}
	}
}

func (c *Client) blockAdmission(delay time.Duration) {
	key := sha256.Sum256([]byte(c.endpoint + "\x00" + c.key))
	until := time.Now().Add(delay)
	admissions.Lock()
	defer admissions.Unlock()
	if until.After(admissions.until[key]) {
		admissions.until[key] = until
	}
}
