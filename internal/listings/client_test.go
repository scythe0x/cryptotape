package listings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func testClient(t *testing.T, handler http.Handler, cb Callbacks) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewClient(Config{Endpoint: FreeEndpoint, Key: "unit-test-key"}, cb)
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = strings.Replace(server.URL, "http://", "ws://", 1)
	return client
}

func writeReady(t *testing.T, ctx context.Context, conn *websocket.Conn) {
	t.Helper()
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"success","code":"READY","subscription":{"plan":"feed_free","delay_ms":3000}}`)); err != nil {
		t.Error(err)
	}
}

func TestClientWaitsForReadyAndReadsOnly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var ready bool
	var seen int
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer unit-test-key" {
			t.Error("missing bearer authorization")
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		writeReady(t, r.Context(), conn)
		raw, err := json.Marshal(listingEvent(t, time.Now()))
		if err != nil {
			t.Error(err)
			return
		}
		if err := conn.Write(r.Context(), websocket.MessageText, raw); err != nil {
			t.Error(err)
			return
		}
		_, message, err := conn.Read(r.Context())
		if err == nil {
			t.Errorf("client sent an application message: %q", message)
		}
	}), Callbacks{
		Ready: func(sub Subscription) { ready = sub.Plan == "feed_free" && *sub.DelayMs == 3000 },
		Event: func(event Event, received time.Time) {
			if !ready || received.IsZero() || event.Parser.Exchange != "upbit" {
				t.Error("event before READY or invalid timestamp")
			}
			seen++
			cancel()
		},
	})
	if err := client.Run(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if !ready || seen != 1 {
		t.Fatalf("ready=%v events=%d", ready, seen)
	}
}

func TestClientDoesNotRetryPermanentAdmissionErrors(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var attempts atomic.Int32
			client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { attempts.Add(1); w.WriteHeader(status) }), Callbacks{})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			var failure *FeedError
			if err := client.Run(ctx); !errors.As(err, &failure) || !failure.Permanent || attempts.Load() != 1 {
				t.Fatalf("permanent error retried: %v", err)
			}
		})
	}
}

func TestClientHonorsRetryAfterAndCancellation(t *testing.T) {
	var attempts atomic.Int32
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Retry-After", "90")
		w.WriteHeader(429)
	}), Callbacks{})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := client.Run(ctx); !errors.Is(err, context.DeadlineExceeded) || attempts.Load() != 1 {
		t.Fatalf("rate limit bypassed: %v attempts=%d", err, attempts.Load())
	}
	if delay := retryAfter(time.Now().Add(time.Minute).UTC().Format(http.TimeFormat), time.Now()); delay < 59*time.Second {
		t.Fatalf("HTTP date lost: %s", delay)
	}
}

func TestClientRejectsEventsBeforeReadyAndPolicyCloses(t *testing.T) {
	for _, mode := range []string{"event_before_ready", "json_auth_error", "policy_close"} {
		t.Run(mode, func(t *testing.T) {
			client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.CloseNow()
				switch mode {
				case "event_before_ready":
					if err := conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"announcement"}`)); err != nil {
						t.Error(err)
					}
				case "json_auth_error":
					if err := conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"error","code":"KEY_EXPIRED","message":"do not expose server messages"}`)); err != nil {
						t.Error(err)
					}
				default:
					writeReady(t, r.Context(), conn)
					conn.Close(websocket.StatusPolicyViolation, "blocked")
				}
			}), Callbacks{Event: func(Event, time.Time) { t.Error("unexpected event") }})
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			var failure *FeedError
			if err := client.Run(ctx); !errors.As(err, &failure) || !failure.Permanent {
				t.Fatalf("protocol failure retried: %v", err)
			}
		})
	}
}

func TestReplacementClientCannotBypassRateLimit(t *testing.T) {
	var attempts atomic.Int32
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(429)
	}), Callbacks{})
	if _, err := client.connection(context.Background()); err == nil {
		t.Fatal("missing rate limit")
	}
	replacement := *client
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := replacement.connection(ctx); !errors.Is(err, context.DeadlineExceeded) || attempts.Load() != 1 {
		t.Fatalf("replacement bypassed limit: %v attempts=%d", err, attempts.Load())
	}
}
