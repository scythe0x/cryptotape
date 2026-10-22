package licensing

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

type Denied struct{ Code string }

func (e *Denied) Error() string { return Message(e.Code) }

type Client struct {
	Endpoint  string
	PublicKey ed25519.PublicKey
	HTTP      *http.Client
	Wait      func(context.Context, time.Duration) error
}

func wait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) Check(ctx context.Context, key string, hardware []string, activate bool) (Grant, error) {
	ctx, cancel := context.WithTimeout(ctx, RetryBudget)
	defer cancel()
	waitFn := c.Wait
	if waitFn == nil {
		waitFn = wait
	}
	var last error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			if err := waitFn(ctx, min(time.Second*time.Duration(1<<attempt), 16*time.Second)); err != nil {
				return Grant{}, err
			}
		}
		grant, err := c.once(ctx, key, hardware, activate)
		if err == nil {
			return grant, nil
		}
		var denied *Denied
		if errors.As(err, &denied) || ctx.Err() != nil {
			return Grant{}, err
		}
		last = err
	}
	return Grant{}, last
}

func (c *Client) once(ctx context.Context, key string, hardware []string, activate bool) (Grant, error) {
	started := Uptime()
	request := Request{Key: NormalizeKey(key), Hardware: hardware, Nonce: NewNonce()}
	if !ValidRequest(request) {
		return Grant{}, &Denied{Code: "INVALID_KEY"}
	}
	body, err := json.Marshal(request)
	if err != nil {
		return Grant{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = Endpoint
	}
	path := "/v1/validate"
	if activate {
		path = "/v1/activate"
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+path, bytes.NewReader(body))
	if err != nil {
		return Grant{}, errors.New("license request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	response, err := client.Do(req)
	if err != nil {
		return Grant{}, errors.New("license transport failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Grant{}, errors.New("license server unavailable")
	}
	var envelope Envelope
	if err = json.NewDecoder(io.LimitReader(response.Body, 8192)).Decode(&envelope); err != nil {
		return Grant{}, &Denied{Code: "UNTRUSTED"}
	}
	g, err := Verify(envelope, c.PublicKey, request)
	if err != nil {
		return Grant{}, &Denied{Code: "UNTRUSTED"}
	}
	if g.Code != "OK" {
		return Grant{}, &Denied{Code: g.Code}
	}
	g.RequestStarted = started
	return g, nil
}
