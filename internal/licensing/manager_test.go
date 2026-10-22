package licensing

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type memoryKeys struct {
	key string
	err error
}

func (k *memoryKeys) Load() (string, error) { return k.key, k.err }
func (k *memoryKeys) Save(key string) error {
	if k.err != nil {
		return k.err
	}
	k.key = key
	return nil
}

type checkFunc func(context.Context, string, []string, bool) (Grant, error)

func (f checkFunc) Check(ctx context.Context, k string, h []string, a bool) (Grant, error) {
	return f(ctx, k, h, a)
}

func TestManagerUsesServerLifetimeAndFailsClosed(t *testing.T) {
	clock := time.Second
	keys := &memoryKeys{}
	failure := false
	m := &Manager{Keys: keys, Hardware: func() ([]string, error) { return []string{"hardware"}, nil }, Clock: func() time.Duration { return clock }}
	m.Client = checkFunc(func(context.Context, string, []string, bool) (Grant, error) {
		if failure {
			return Grant{}, &Denied{Code: "REVOKED"}
		}
		return Grant{Code: "OK", ServerTime: 100, ExpiresAt: 110}, nil
	})
	if m.Allowed() {
		t.Fatal("access before validation")
	}
	if s := m.Authenticate(context.Background(), "key", true); s.Phase != "ready" || !m.Allowed() {
		t.Fatal(s)
	}
	clock += 11 * time.Second
	if m.Allowed() {
		t.Fatal("access past server lifetime")
	}
	clock = time.Second
	failure = true
	if s := m.Retry(context.Background()); s.Code != "REVOKED" || m.Allowed() {
		t.Fatal(s)
	}
}
func TestKeyStorageAndHardwareFailuresDoNotOpenAccess(t *testing.T) {
	for _, hardwareFailure := range []bool{false, true} {
		keys := &memoryKeys{err: errors.New("storage")}
		m := &Manager{Keys: keys, Hardware: func() ([]string, error) {
			if hardwareFailure {
				return nil, errors.New("hardware")
			}
			return []string{"pc"}, nil
		}, Client: checkFunc(func(context.Context, string, []string, bool) (Grant, error) {
			return Grant{Code: "OK", ServerTime: 1, ExpiresAt: 1000}, nil
		})}
		s := m.Authenticate(context.Background(), "key", true)
		expected := "STORAGE"
		if hardwareFailure {
			expected = "HARDWARE"
		}
		if s.Code != expected || m.Allowed() {
			t.Fatal(s)
		}
	}
}
func TestBlockedInFlightValidationCannotRestoreAccess(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	m := &Manager{Keys: &memoryKeys{}, Hardware: func() ([]string, error) { return []string{"pc"}, nil }, Client: checkFunc(func(context.Context, string, []string, bool) (Grant, error) {
		close(entered)
		<-release
		return Grant{Code: "OK", ServerTime: 100, ExpiresAt: 1000}, nil
	})}
	done := make(chan struct{})
	go func() { m.Authenticate(context.Background(), "key", true); close(done) }()
	<-entered
	m.Block("REVOKED")
	close(release)
	<-done
	if m.Allowed() {
		t.Fatal("stale request restored access")
	}
}
func TestExpiryWatchdogRunsDuringHeartbeatRetry(t *testing.T) {
	var clock atomic.Int64
	clock.Store(int64(time.Second))
	var calls atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := &Manager{Keys: &memoryKeys{key: "key"}, Clock: func() time.Duration { return time.Duration(clock.Load()) }, Hardware: func() ([]string, error) { return []string{"pc"}, nil }}
	m.Client = checkFunc(func(context.Context, string, []string, bool) (Grant, error) {
		if calls.Add(1) == 1 {
			return Grant{Code: "OK", ServerTime: 100, ExpiresAt: 100 + 3601}, nil
		}
		close(entered)
		<-release
		return Grant{}, errors.New("network")
	})
	done := make(chan struct{})
	go func() { m.Start(ctx); close(done) }()
	deadline := time.After(2 * time.Second)
	for !m.Allowed() {
		select {
		case <-deadline:
			t.Fatal("startup timed out")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	clock.Store(int64(CheckInterval - time.Second))
	time.Sleep(300 * time.Millisecond)
	clock.Store(int64(CheckInterval + time.Second))
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("no heartbeat")
	}
	clock.Store(int64(CheckInterval + 3*time.Second))
	time.Sleep(300 * time.Millisecond)
	if m.Allowed() || m.State().Code != "EXPIRED" {
		t.Fatal("expiry ignored during network retry")
	}
	close(release)
	cancel()
	<-done
}
