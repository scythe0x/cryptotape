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
	"time"
)

var validSymbol = regexp.MustCompile(`^[A-Z0-9_]+$`)

type Terminal struct {
	client  *http.Client
	baseURL string
}

func NewTerminal() *Terminal {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &Terminal{
		client:  &http.Client{Transport: transport, Timeout: 1500 * time.Millisecond},
		baseURL: "http://localhost:5005",
	}
}

func (t *Terminal) Open(ctx context.Context, symbol string) error {
	if !validSymbol.MatchString(symbol) {
		return errors.New("invalid Binance symbol")
	}
	endpoint := t.baseURL + "/api/open/zenica/binance/F/" + url.PathEscape(symbol)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := t.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
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
