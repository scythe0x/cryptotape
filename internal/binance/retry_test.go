package binance

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"testing/synctest"
	"time"
)

func TestRetriesStopAfterOneTwoFourEightSeconds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		for range 2 {
			start := time.Now()
			var attempts []time.Duration
			var retries []bool
			err := RetryConnection(context.Background(), func() (bool, error) {
				attempts = append(attempts, time.Since(start))
				return false, errors.New("offline")
			}, func(_ error, retry bool) { retries = append(retries, retry) })
			if !errors.Is(err, ErrRetriesExhausted) {
				t.Fatalf("unexpected result: %v", err)
			}
			expected := []time.Duration{0, time.Second, 3 * time.Second, 7 * time.Second, 15 * time.Second}
			if !reflect.DeepEqual(attempts, expected) {
				t.Fatalf("retry times: %v", attempts)
			}
			if !reflect.DeepEqual(retries, []bool{true, true, true, true, false}) {
				t.Fatalf("retry states: %v", retries)
			}
		}
	})
}

func TestCanceledRetryAndNestedExhaustionDoNotRestart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0
		err := RetryConnection(ctx, func() (bool, error) { attempts++; return false, errors.New("offline") }, func(error, bool) { cancel() })
		if !errors.Is(err, context.Canceled) || attempts != 1 {
			t.Fatalf("cancellation ignored: %v, %d", err, attempts)
		}
		start := time.Now()
		attempts = 0
		err = RetryConnection(context.Background(), func() (bool, error) { attempts++; return false, ErrRetriesExhausted }, func(error, bool) {})
		if !errors.Is(err, ErrRetriesExhausted) || attempts != 1 || time.Since(start) != 0 {
			t.Fatal("outer retry multiplied an exhausted stream budget")
		}
	})
}

func TestSlowFailedHandshakesDoNotResetRetryBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		attempts := 0
		err := RetryConnection(context.Background(), func() (bool, error) {
			attempts++
			time.Sleep(2 * time.Minute)
			return false, errors.New("handshake timed out")
		}, func(error, bool) {})
		if !errors.Is(err, ErrRetriesExhausted) || attempts != 5 {
			t.Fatal("slow failed connections retried indefinitely")
		}
	})
}

func TestStableSessionResetsConsecutiveFailureBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		attempts := 0
		var times []time.Time
		err := RetryConnection(context.Background(), func() (bool, error) {
			attempts++
			times = append(times, time.Now())
			if attempts == 3 {
				time.Sleep(time.Minute)
				return true, errors.New("disconnected after healthy session")
			}
			return false, errors.New("offline")
		}, func(error, bool) {})
		if !errors.Is(err, ErrRetriesExhausted) || attempts != 7 {
			t.Fatalf("stable session did not reset budget: %d", attempts)
		}
		if times[3].Sub(times[2]) != time.Minute+time.Second {
			t.Fatal("new outage did not restart at a one-second delay")
		}
	})
}
