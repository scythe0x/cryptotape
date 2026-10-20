package binance

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestClockSyncExcludesAdmissionAndConnectionSetup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"serverTime":%d}`, time.Now().UnixMilli())
	}))
	defer server.Close()
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	transport.Proxy = nil
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if !pause(ctx, 600*time.Millisecond) {
			return nil, context.Cause(ctx)
		}
		return dialer.DialContext(ctx, network, address)
	}
	gate := &restGate{token: make(chan struct{}, 1)}
	gate.token <- struct{}{}
	timer := time.AfterFunc(600*time.Millisecond, func() { <-gate.token })
	defer timer.Stop()
	f := &Feed{client: &http.Client{Transport: streamTransport{target: target, base: transport}}, rest: gate}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := f.syncClock(ctx); err != nil {
		t.Fatal(err)
	}
	if offset := time.Duration(f.clockOffset.Load()); offset < -200*time.Millisecond || offset > 200*time.Millisecond {
		t.Fatalf("connection setup or REST admission biased clock offset: %s", offset)
	}
}
