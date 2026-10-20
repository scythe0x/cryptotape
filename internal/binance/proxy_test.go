package binance

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"cryptotape/internal/core"
	"github.com/coder/websocket"
)

func TestAuthenticatedConnectProxyCarriesRESTAndWebSocket(t *testing.T) {
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/socket" {
			connection, err := websocket.Accept(w, r, nil)
			if err != nil {
				t.Error(err)
				return
			}
			defer connection.CloseNow()
			if err := connection.Write(r.Context(), websocket.MessageText, []byte("market tick")); err != nil {
				t.Error(err)
			}
			return
		}
		_, _ = io.WriteString(w, "market snapshot")
	}))
	defer target.Close()
	var connections atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("test-user:test-password"))
		if r.Method != http.MethodConnect || r.Header.Get("Proxy-Authorization") != expected || r.Host != strings.TrimPrefix(target.URL, "https://") {
			t.Error("missing CONNECT destination or proxy credentials")
			w.WriteHeader(http.StatusProxyAuthRequired)
			return
		}
		upstream, err := net.DialTimeout("tcp", r.Host, time.Second)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer upstream.Close()
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("test server cannot tunnel")
			return
		}
		downstream, buffer, err := hijacker.Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer downstream.Close()
		connections.Add(1)
		if _, err := buffer.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			t.Error(err)
			return
		}
		if err := buffer.Flush(); err != nil {
			t.Error(err)
			return
		}
		done := make(chan struct{})
		go func() { io.Copy(upstream, buffer); upstream.Close(); close(done) }()
		io.Copy(downstream, upstream)
		downstream.Close()
		<-done
	}))
	defer proxy.Close()
	address := "http://test-user:test-password@" + strings.TrimPrefix(proxy.URL, "http://")
	feed, err := NewFeed(address, 0, Callbacks{})
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := feed.client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("unexpected transport")
	}
	defer transport.CloseIdleConnections()
	trusted, ok := target.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatal("unexpected test TLS transport")
	}
	transport.TLSClientConfig = trusted.TLSClientConfig.Clone()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := feed.client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || string(body) != "market snapshot" {
		t.Fatal("REST did not cross authenticated tunnel")
	}
	socket, _, err := websocket.Dial(ctx, "wss"+strings.TrimPrefix(target.URL, "https")+"/socket", &websocket.DialOptions{HTTPClient: feed.client})
	if err != nil {
		t.Fatal(err)
	}
	defer socket.CloseNow()
	_, raw, err := socket.Read(ctx)
	if err != nil || string(raw) != "market tick" {
		t.Fatalf("websocket failed through proxy: %v", err)
	}
	if connections.Load() < 1 {
		t.Fatal("proxy bypassed")
	}
}

func TestSOCKS5CredentialsAreUsedWithoutDirectFallback(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	authenticated := make(chan bool, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		connection.SetDeadline(time.Now().Add(2 * time.Second))
		greeting := make([]byte, 2)
		if _, err := io.ReadFull(connection, greeting); err != nil {
			return
		}
		methods := make([]byte, int(greeting[1]))
		if _, err := io.ReadFull(connection, methods); err != nil {
			return
		}
		if _, err := connection.Write([]byte{5, 2}); err != nil {
			return
		}
		header := make([]byte, 2)
		if _, err := io.ReadFull(connection, header); err != nil {
			return
		}
		login := make([]byte, int(header[1]))
		if _, err := io.ReadFull(connection, login); err != nil {
			return
		}
		length := []byte{0}
		if _, err := io.ReadFull(connection, length); err != nil {
			return
		}
		password := make([]byte, int(length[0]))
		if _, err := io.ReadFull(connection, password); err != nil {
			return
		}
		authenticated <- string(login) == "test-user" && string(password) == "test-password"
		// reject the session: the transport must return an error rather than dial directly.
		connection.Write([]byte{1, 1})
	}()
	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	var directRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { directRequests.Add(1); w.WriteHeader(http.StatusOK) }))
	defer target.Close()
	profile := core.ProxyProfile{Host: "127.0.0.1", Port: port, Protocol: "socks5", Login: "test-user", Password: "test-password"}
	address, err := profile.URL()
	if err != nil {
		t.Fatal(err)
	}
	feed, err := NewFeed(address, 0, Callbacks{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response, err := feed.client.Do(request); err == nil {
		response.Body.Close()
		t.Fatal("rejected SOCKS authentication was ignored")
	}
	<-done
	if directRequests.Load() != 0 {
		t.Fatal("SOCKS failure fell back to a direct route")
	}
	select {
	case ok := <-authenticated:
		if !ok {
			t.Fatal("incorrect SOCKS credentials")
		}
	default:
		t.Fatal("SOCKS authentication not attempted")
	}
}
