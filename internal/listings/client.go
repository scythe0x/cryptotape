package listings

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
)

type Subscription struct {
	Plan    string `json:"plan"`
	DelayMs *int64 `json:"delay_ms"`
}

type Callbacks struct {
	Ready  func(Subscription)
	Event  func(Event, time.Time)
	Status func(string, string)
}

type Client struct {
	endpoint string
	key      string
	http     *http.Client
	cb       Callbacks
}

type FeedError struct {
	Code       string
	Permanent  bool
	RetryAfter time.Duration
}

func (e *FeedError) Error() string { return e.Code }

func NewClient(config Config, cb Callbacks) (*Client, error) {
	if !ValidEndpoint(config.Endpoint) || config.Key == "" || !validKey(config.Key) {
		return nil, errors.New("invalid listing source config")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &Client{endpoint: config.Endpoint + "?category=crypto", key: config.Key, http: &http.Client{Transport: transport}, cb: cb}, nil
}

func (c *Client) status(state, code string) {
	if c.cb.Status != nil {
		c.cb.Status(state, code)
	}
}

func (c *Client) Run(ctx context.Context) error {
	defer c.http.CloseIdleConnections()
	backoff := time.Second
	for ctx.Err() == nil {
		c.status("connecting", "")
		ready, err := c.connection(ctx)
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}
		var failure *FeedError
		if !errors.As(err, &failure) {
			failure = &FeedError{Code: "CONNECTION_LOST"}
		}
		if failure.Permanent {
			c.status("error", failure.Code)
			return failure
		}
		if ready {
			backoff = time.Second
		}
		delay := max(backoff, failure.RetryAfter) + time.Duration(rand.Int64N(int64(250*time.Millisecond)))
		c.status("reconnecting", failure.Code)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return context.Cause(ctx)
		case <-timer.C:
		}
		backoff = min(backoff*2, 30*time.Second)
	}
	return context.Cause(ctx)
}

func (c *Client) connection(ctx context.Context) (bool, error) {
	if err := c.waitAdmission(ctx); err != nil {
		return false, err
	}
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	conn, response, err := websocket.Dial(dialCtx, c.endpoint, &websocket.DialOptions{HTTPClient: c.http, HTTPHeader: http.Header{"Authorization": []string{"Bearer " + c.key}}})
	cancel()
	if err != nil {
		failure := &FeedError{Code: "CONNECTION_FAILED"}
		if response != nil {
			if response.Body != nil {
				response.Body.Close()
			}
			failure.Code = "HTTP_" + strconv.Itoa(response.StatusCode)
			failure.Permanent = response.StatusCode >= 400 && response.StatusCode < 500 && response.StatusCode != 408 && response.StatusCode != 429
			failure.RetryAfter = retryAfter(response.Header.Get("Retry-After"), time.Now())
			if response.StatusCode == http.StatusTooManyRequests {
				if failure.RetryAfter == 0 {
					failure.RetryAfter = time.Minute
				}
				c.blockAdmission(failure.RetryAfter)
			}
		}
		return false, failure
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)
	readyCtx, stopReady := context.WithTimeout(ctx, 5*time.Second)
	_, raw, err := conn.Read(readyCtx)
	stopReady()
	if err != nil {
		return false, connectionError(err)
	}
	var message struct {
		Type         string       `json:"type"`
		Code         string       `json:"code"`
		Subscription Subscription `json:"subscription"`
	}
	if json.Unmarshal(raw, &message) != nil {
		return false, &FeedError{Code: "INVALID_JSON", Permanent: true}
	}
	if message.Type == "error" {
		return false, applicationError(message.Code)
	}
	if message.Type != "success" || message.Code != "READY" || message.Subscription.Plan == "" || message.Subscription.DelayMs == nil || *message.Subscription.DelayMs < 0 {
		return false, &FeedError{Code: "INVALID_READY", Permanent: true}
	}
	if c.cb.Ready != nil {
		c.cb.Ready(message.Subscription)
	}
	c.status("connected", "")
	return true, c.readEvents(ctx, conn)
}

func (c *Client) readEvents(ctx context.Context, conn *websocket.Conn) error {
	// protocol pings are allowed; no application messages or JSON heartbeats are sent.
	pingCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		timer := time.NewTicker(30 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-pingCtx.Done():
				return
			case <-timer.C:
				timeout, stop := context.WithTimeout(pingCtx, 10*time.Second)
				err := conn.Ping(timeout)
				stop()
				if err != nil {
					conn.CloseNow()
					return
				}
			}
		}
	}()
	defer func() { cancel(); <-done }()
	for {
		_, raw, err := conn.Read(ctx)
		received := time.Now()
		if err != nil {
			return connectionError(err)
		}
		var message struct {
			Event
			Code string `json:"code"`
		}
		if json.Unmarshal(raw, &message) != nil {
			return &FeedError{Code: "INVALID_JSON", Permanent: true}
		}
		if message.Type == "error" {
			return applicationError(message.Code)
		}
		if message.Type == "announcement" || message.Type == "tweet" {
			if c.cb.Event != nil {
				c.cb.Event(message.Event, received)
			}
		} else {
			return &FeedError{Code: "UNEXPECTED_MESSAGE", Permanent: true}
		}
	}
}

func applicationError(code string) *FeedError {
	switch code {
	case "SERVER_UNAVAILABLE":
		return &FeedError{Code: code}
	case "CONNECTION_LIMIT_REACHED", "CONNECTION_CLOSED", "AUTHENTICATION_FAILED", "KEY_EXPIRED", "INVALID_REQUEST", "RATE_LIMITED":
		return &FeedError{Code: code, Permanent: true}
	default:
		return &FeedError{Code: "UNKNOWN_SERVER_ERROR", Permanent: true}
	}
}

func connectionError(err error) *FeedError {
	if websocket.CloseStatus(err) == websocket.StatusPolicyViolation {
		return &FeedError{Code: "POLICY_VIOLATION", Permanent: true}
	}
	return &FeedError{Code: "CONNECTION_LOST"}
}

func retryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= int64((365*24*time.Hour)/time.Second) {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil {
		return max(0, at.Sub(now))
	}
	return 0
}
