package binance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RateLimitError struct {
	Status int
	Until  time.Time
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("binance HTTP %d: requests paused until %s", e.Status, e.Until.UTC().Format(time.RFC3339))
}

type restGate struct {
	token   chan struct{}
	blocked time.Time
}

var routeGates = struct {
	sync.Mutex
	gates map[string]*restGate
}{gates: make(map[string]*restGate)}

func gateForRoute(route string) *restGate {
	routeGates.Lock()
	defer routeGates.Unlock()
	gate := routeGates.gates[route]
	if gate == nil {
		gate = &restGate{token: make(chan struct{}, 1)}
		routeGates.gates[route] = gate
	}
	return gate
}

func (f *Feed) get(ctx context.Context, path string, out any) error {
	f.mu.Lock()
	if f.rest == nil {
		f.rest = &restGate{token: make(chan struct{}, 1)}
	}
	gate := f.rest
	f.mu.Unlock()
	// share admission across services and replacement feeds on the same route.
	for {
		select {
		case gate.token <- struct{}{}:
		case <-ctx.Done():
			return context.Cause(ctx)
		}
		if delay := time.Until(gate.blocked); delay > 0 {
			<-gate.token
			if !pause(ctx, delay) {
				return context.Cause(ctx)
			}
			continue
		}
		break
	}
	defer func() { <-gate.token }()
	requestCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, restBase+path, nil)
	if err != nil {
		return err
	}
	response, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if weight, err := strconv.ParseInt(response.Header.Get("X-Mbx-Used-Weight-1m"), 10, 64); err == nil {
		f.mu.Lock()
		f.usedWeight = weight
		f.mu.Unlock()
	}
	if response.StatusCode != http.StatusOK {
		body, err := io.ReadAll(io.LimitReader(response.Body, 4096))
		if err != nil {
			return err
		}
		if response.StatusCode == 429 || response.StatusCode == 418 {
			gate.blocked = retryDeadline(response.Header.Get("Retry-After"), body, response.StatusCode, time.Now())
			return &RateLimitError{Status: response.StatusCode, Until: gate.blocked}
		}
		return fmt.Errorf("binance %s: %.256s", response.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(out)
}

func retryDeadline(header string, body []byte, status int, now time.Time) time.Time {
	fallback := time.Minute
	// without a server deadline, avoid repeatedly probing a potentially multi-day ban.
	if status == 418 {
		fallback = 72 * time.Hour
	}
	until := now.Add(fallback)
	explicit := false
	if seconds, err := strconv.ParseFloat(header, 64); err == nil && seconds >= 0 {
		until, explicit = now.Add(time.Duration(seconds*float64(time.Second))), true
	} else if parsed, err := http.ParseTime(header); err == nil && parsed.After(now) {
		until, explicit = parsed, true
	}
	var payload struct {
		RetryAfter int64  `json:"retryAfter"`
		Msg        string `json:"msg"`
	}
	if json.Unmarshal(body, &payload) == nil {
		if payload.RetryAfter > now.UnixMilli() {
			parsed := time.UnixMilli(payload.RetryAfter)
			if !explicit || parsed.After(until) {
				until = parsed
			}
			explicit = true
		}
		// Binance also embeds the ban expiration in its error message.
		for _, word := range strings.Fields(payload.Msg) {
			value, err := strconv.ParseInt(strings.Trim(word, ".,;"), 10, 64)
			if err == nil && value > now.UnixMilli() && value < now.Add(7*24*time.Hour).UnixMilli() {
				parsed := time.UnixMilli(value)
				if !explicit || parsed.After(until) {
					until = parsed
					explicit = true
				}
			}
		}
	}
	return until
}

func (f *Feed) refreshServices() map[string]func(context.Context) error {
	return map[string]func(context.Context) error{
		"clock": f.syncClock,
		"symbols": func(ctx context.Context) error {
			symbols, err := f.loadSymbols(ctx)
			if err == nil {
				f.refreshSymbols(symbols)
			}
			return err
		},
		"turnover": func(ctx context.Context) error {
			volumes, err := f.loadVolumes(ctx)
			if err == nil {
				f.cb.Volume(volumes)
				f.updateEligibility(volumes)
				f.mu.Lock()
				f.volumeUpdated = time.Now()
				f.mu.Unlock()
			}
			return err
		},
	}
}

func (f *Feed) refreshLoop(ctx context.Context) {
	var wg sync.WaitGroup
	for name, refresh := range f.refreshServices() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !pause(ctx, 5*time.Minute) {
				return
			}
			for ctx.Err() == nil {
				var err error
				for attempt := 0; attempt < 5; attempt++ {
					err = refresh(ctx)
					f.mu.Lock()
					if err == nil {
						delete(f.serviceErrors, name)
					} else {
						f.serviceErrors[name] = err.Error()
					}
					f.mu.Unlock()
					if err == nil || ctx.Err() != nil {
						break
					}
					var limited *RateLimitError
					delay := time.Second << attempt
					if errors.As(err, &limited) {
						delay = max(delay, time.Until(limited.Until))
					}
					if !pause(ctx, delay) {
						return
					}
				}
				if !pause(ctx, 5*time.Minute) {
					return
				}
			}
		}()
	}
	wg.Wait()
}
