package licensing

import (
	"context"
	"errors"
	"sync"
	"time"
)

type State struct {
	Phase    string `json:"phase"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	BotURL   string `json:"botUrl"`
	CanRetry bool   `json:"canRetry"`
}
type KeyStore interface {
	Load() (string, error)
	Save(string) error
}
type Checker interface {
	Check(context.Context, string, []string, bool) (Grant, error)
}

type Manager struct {
	mu         sync.Mutex
	checkMu    sync.Mutex
	Client     Checker
	Keys       KeyStore
	Hardware   func() ([]string, error)
	Changed    func(State)
	Clock      func() time.Duration
	state      State
	key        string
	allowed    bool
	deadline   time.Duration
	due        time.Duration
	generation uint64
}

func (m *Manager) now() time.Duration {
	if m.Clock != nil {
		return m.Clock()
	}
	return Uptime()
}
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.state
	if s.Phase == "" {
		s.Phase = "loading"
	}
	s.BotURL = BotURL
	s.CanRetry = m.key != ""
	return s
}
func (m *Manager) Allowed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.allowed && m.now() < m.deadline
}
func (m *Manager) publish(phase, code string) {
	m.mu.Lock()
	m.state = State{Phase: phase, Code: code, Message: Message(code), BotURL: BotURL, CanRetry: m.key != ""}
	s := m.state
	m.mu.Unlock()
	if m.Changed != nil {
		m.Changed(s)
	}
}
func (m *Manager) Block(code string) {
	m.mu.Lock()
	m.allowed = false
	m.generation++
	m.mu.Unlock()
	m.publish("locked", code)
}

func (m *Manager) Start(ctx context.Context) {
	key, err := m.Keys.Load()
	m.mu.Lock()
	m.key = key
	m.mu.Unlock()
	done := make(chan struct{}, 1)
	checking := false
	launch := func(key string) {
		checking = true
		go func() { m.Authenticate(ctx, key, false); done <- struct{}{} }()
	}
	if err != nil {
		m.publish("locked", "STORAGE")
	} else if key == "" {
		m.publish("locked", "")
	} else {
		launch(key)
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	last := m.now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			checking = false
		case <-ticker.C:
			now := m.now()
			m.mu.Lock()
			active, due, expiry, key := m.allowed, m.due, m.deadline, m.key
			m.mu.Unlock()
			if active && now >= expiry {
				m.Block("EXPIRED")
			} else if active && now >= due && !checking {
				if now-last > 5*time.Second {
					m.Block("NETWORK")
				}
				launch(key)
			}
			last = now
		}
	}
}

func (m *Manager) Retry(ctx context.Context) State {
	m.mu.Lock()
	key := m.key
	m.mu.Unlock()
	if key == "" {
		return m.State()
	}
	return m.Authenticate(ctx, key, false)
}

func (m *Manager) Authenticate(ctx context.Context, key string, activate bool) State {
	m.checkMu.Lock()
	defer m.checkMu.Unlock()
	m.mu.Lock()
	generation := m.generation
	wasAllowed := m.allowed
	m.mu.Unlock()
	if !wasAllowed {
		m.publish("loading", "")
	}
	hardware, err := m.Hardware()
	if err != nil {
		m.Block("HARDWARE")
		return m.State()
	}
	started := m.now()
	g, err := m.Client.Check(ctx, key, hardware, activate)
	if err != nil {
		if ctx.Err() != nil {
			return m.State()
		}
		code := "NETWORK"
		var denied *Denied
		if errors.As(err, &denied) {
			code = denied.Code
		}
		m.Block(code)
		return m.State()
	}
	// subtract the successful request round trip instead of trusting the local wall clock.
	if g.RequestStarted != 0 && m.Clock == nil {
		started = g.RequestStarted
	}
	deadline := started + time.Duration(g.ExpiresAt-g.ServerTime)*time.Second
	if m.now() >= deadline {
		m.Block("EXPIRED")
		return m.State()
	}
	if err = m.Keys.Save(key); err != nil {
		m.Block("STORAGE")
		return m.State()
	}
	m.mu.Lock()
	if generation != m.generation {
		m.mu.Unlock()
		return m.State()
	}
	m.key = key
	m.allowed = true
	m.deadline = deadline
	m.due = m.now() + CheckInterval
	m.mu.Unlock()
	m.publish("ready", "")
	return m.State()
}
