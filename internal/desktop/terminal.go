package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var errSymbolUnavailable = errors.New("symbol unavailable")

var validSymbol = regexp.MustCompile(`^[A-Z0-9_]+$`)

type Terminal struct {
	client  *http.Client
	baseURL string
}

func NewTerminal() *Terminal {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.MaxIdleConnsPerHost = 32
	transport.MaxConnsPerHost = 32
	return &Terminal{
		client:  &http.Client{Transport: transport, Timeout: 1500 * time.Millisecond},
		baseURL: "http://localhost:5005",
	}
}

func (t *Terminal) Open(ctx context.Context, symbol string) error {
	return t.OpenExchange(ctx, symbol, "binance")
}

func (t *Terminal) OpenExchange(ctx context.Context, symbol, exchange string) error {
	if exchange != "binance" && exchange != "bybit" && exchange != "okx" && exchange != "gate" {
		return errors.New("unknown exchange")
	}
	if !validSymbol.MatchString(symbol) {
		return errors.New("invalid symbol")
	}
	target := symbol
	if exchange == "okx" || exchange == "gate" {
		if !strings.HasSuffix(symbol, "USDT") || len(symbol) <= 4 {
			return errors.New("invalid USDT futures symbol")
		}
		asset := strings.TrimSuffix(symbol, "USDT")
		if exchange == "okx" {
			target = asset + "-USDT-SWAP"
		} else {
			target = asset + "_USDT"
		}
	}
	endpoint := t.baseURL + "/api/open/zenica/" + exchange + "/F/" + url.PathEscape(target)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := t.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return errSymbolUnavailable
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("terminal returned %s", response.Status)
	}
	var result struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&result); err != nil {
		return err
	}
	if result.Status != "ok" {
		return fmt.Errorf("terminal returned status %q", result.Status)
	}
	return nil
}
