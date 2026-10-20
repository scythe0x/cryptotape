package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/coder/websocket"
)

type Tiger struct {
	mu       sync.Mutex
	conn     *websocket.Conn
	endpoint string
	client   *http.Client
	pending  map[string]chan error
	sequence atomic.Uint64
}

func NewTiger() *Tiger {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &Tiger{client: &http.Client{Transport: transport}, pending: make(map[string]chan error)}
}

func (t *Tiger) Open(ctx context.Context, symbol string, port int, group string) error {
	if !validSymbol.MatchString(symbol) {
		return errors.New("invalid Binance symbol")
	}
	endpoint := "ws://127.0.0.1:" + strconv.Itoa(port) + "/"
	t.mu.Lock()
	if t.conn != nil && endpoint != t.endpoint {
		t.disconnectLocked(errors.New("Tiger endpoint changed"))
	}
	if err := ctx.Err(); err != nil {
		t.mu.Unlock()
		return err
	}
	if t.conn == nil {
		connection, _, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{HTTPClient: t.client})
		if err != nil {
			t.mu.Unlock()
			return fmt.Errorf("Tiger: %w", err)
		}
		t.conn, t.endpoint = connection, endpoint
		connection.SetReadLimit(1 << 20)
		go t.read(connection)
	}
	connection := t.conn
	id := strconv.FormatUint(t.sequence.Add(1), 10)
	result := make(chan error, 1)
	t.pending[id] = result
	t.mu.Unlock()
	defer func() { t.mu.Lock(); delete(t.pending, id); t.mu.Unlock() }()
	request := struct {
		Type      string `json:"type"`
		RequestID string `json:"requestId"`
		Exchange  string `json:"e"`
		Market    string `json:"m"`
		Symbol    string `json:"symbol"`
		Group     string `json:"linkGroup"`
	}{"SETLINKSYMBOL", id, "BINANCE-FUT", "FUTURES", symbol, group}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	if err = connection.Write(ctx, websocket.MessageText, data); err != nil {
		t.fail(connection, err)
		return err
	}
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *Tiger) read(connection *websocket.Conn) {
	for {
		_, raw, err := connection.Read(context.Background())
		if err != nil {
			t.fail(connection, err)
			return
		}
		if string(raw) == "pong" {
			continue
		}
		var response struct {
			RequestID string `json:"requestId"`
			Response  struct {
				Type  string `json:"type"`
				Error struct {
					Message string `json:"message"`
					Code    int    `json:"code"`
				} `json:"error"`
			} `json:"response"`
		}
		if err = json.Unmarshal(raw, &response); err != nil {
			t.fail(connection, fmt.Errorf("invalid Tiger response: %w", err))
			return
		}
		if response.RequestID == "" {
			continue
		}
		if response.Response.Type != "successfully" {
			err = fmt.Errorf("Tiger %d: %s", response.Response.Error.Code, response.Response.Error.Message)
		}
		t.mu.Lock()
		if result := t.pending[response.RequestID]; result != nil {
			delete(t.pending, response.RequestID)
			result <- err
		}
		t.mu.Unlock()
	}
}

func (t *Tiger) disconnectLocked(err error) {
	if t.conn != nil {
		t.conn.CloseNow()
		t.conn = nil
	}
	for id, result := range t.pending {
		result <- err
		delete(t.pending, id)
	}
}
func (t *Tiger) fail(connection *websocket.Conn, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn == connection {
		t.disconnectLocked(err)
	}
}
func (t *Tiger) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.disconnectLocked(errors.New("Tiger client closed"))
}
