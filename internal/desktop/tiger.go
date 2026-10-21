package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/coder/websocket"
)

type Tiger struct {
	mu       sync.Mutex
	conn     *websocket.Conn
	endpoint string
	client   *http.Client
	pending  map[string]tigerRequest
	sequence atomic.Uint64
}

type tigerRequest struct {
	symbol, exchange, group string
	result                  chan error
}

func NewTiger() *Tiger {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &Tiger{client: &http.Client{Transport: transport}, pending: make(map[string]tigerRequest)}
}

func (t *Tiger) Open(ctx context.Context, symbol string, port int, group string) error {
	return t.OpenExchange(ctx, symbol, "binance", port, group)
}

func (t *Tiger) OpenExchange(ctx context.Context, symbol, exchange string, port int, group string) error {
	exchanges := map[string]string{"binance": "BINANCE-FUT", "bybit": "BYBIT", "okx": "OKX", "gate": "Gate.io"}
	name, ok := exchanges[exchange]
	if !ok {
		return errors.New("unknown exchange")
	}
	if !validSymbol.MatchString(symbol) {
		return errors.New("invalid symbol")
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
	t.pending[id] = tigerRequest{symbol: symbol, exchange: name, group: group, result: result}
	t.mu.Unlock()
	defer func() { t.mu.Lock(); delete(t.pending, id); t.mu.Unlock() }()
	request := struct {
		Type      string `json:"type"`
		RequestID string `json:"requestId"`
		Exchange  string `json:"e"`
		Market    string `json:"m"`
		Symbol    string `json:"symbol"`
		Group     string `json:"linkGroup"`
	}{"SETLINKSYMBOL", id, name, "FUTURES", symbol, group}
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
		t.fail(connection, ctx.Err())
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
			Request   struct {
				Symbol   string `json:"symbol"`
				Exchange string `json:"e"`
				Group    string `json:"linkGroup"`
				Market   string `json:"m"`
			} `json:"request"`
			Response struct {
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
		if response.Response.Type != "successfully" {
			err = fmt.Errorf("Tiger %d: %s", response.Response.Error.Code, response.Response.Error.Message)
			message := strings.ToLower(response.Response.Error.Message)
			if response.Response.Error.Code == 400 && (strings.Contains(message, "symbol") || strings.Contains(message, "instrument")) && (strings.Contains(message, "not found") || strings.Contains(message, "unknown") || strings.Contains(message, "not exist")) {
				err = fmt.Errorf("%w: %v", errSymbolUnavailable, err)
			}
		}
		t.mu.Lock()
		id := response.RequestID
		if id == "" && strings.EqualFold(response.Request.Market, "FUTURES") {
			for candidate, request := range t.pending {
				if strings.EqualFold(request.symbol, response.Request.Symbol) && strings.EqualFold(request.exchange, response.Request.Exchange) && strings.EqualFold(request.group, response.Request.Group) {
					if id != "" {
						id = ""
						break
					}
					id = candidate
				}
			}
		}
		if request, ok := t.pending[id]; ok {
			delete(t.pending, id)
			request.result <- err
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
		result.result <- err
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
