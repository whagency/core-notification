package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestSenderSend_WritesJSONToWebsocket(t *testing.T) {
	t.Parallel()

	received := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()

		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read message: %v", err)
			return
		}
		received <- string(msg)
	}))
	defer server.Close()

	sender := NewSender(mustWebsocketURL(t, server.URL))
	err := sender.Send(context.Background(), map[string]any{
		"t": "sd",
		"u": 42,
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	select {
	case got := <-received:
		var payload map[string]any
		if err := json.Unmarshal([]byte(got), &payload); err != nil {
			t.Fatalf("unmarshal payload %q: %v", got, err)
		}
		if payload["t"] != "sd" || payload["u"] != float64(42) {
			t.Fatalf("unexpected payload: %#v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for websocket payload")
	}
}

func TestSenderSend_ReturnsMarshalError(t *testing.T) {
	t.Parallel()

	sender := NewSender(url.URL{Scheme: "ws", Host: "127.0.0.1:1", Path: "/ws"})
	err := sender.Send(context.Background(), map[string]any{
		"bad": make(chan int),
	})
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "unsupported type") {
		t.Fatalf("expected marshal cause, got %v", err)
	}
}

func TestSenderSend_ReturnsConnectionError(t *testing.T) {
	t.Parallel()

	sender := NewSender(url.URL{Scheme: "ws", Host: "127.0.0.1:1", Path: "/ws"})
	err := sender.Send(context.Background(), map[string]string{"t": "sd"})
	if err == nil {
		t.Fatal("expected connection error")
	}
	if !strings.Contains(err.Error(), "getConnection") {
		t.Fatalf("expected getConnection context, got %v", err)
	}
}

func mustWebsocketURL(t testing.TB, raw string) url.URL {
	t.Helper()

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse test server url: %v", err)
	}
	u.Scheme = "ws"
	return *u
}
