package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"notification-service/internal/domain"

	"github.com/gorilla/websocket"
	"github.com/whagency/core-common/logger"
)

func TestMain(m *testing.M) {
	if err := logger.Init(logger.Config{
		AppName:     "notification-service",
		Environment: "test",
		Level:       "error",
	}); err != nil {
		panic(err)
	}

	code := m.Run()
	logger.Sync()
	os.Exit(code)
}

func TestRoomRemoveChatter_IsIdempotent(t *testing.T) {
	t.Parallel()

	room := NewWsRoom("public")
	chatter := &Chatter{
		Uid:  1,
		Send: make(chan []byte, 1),
		Room: room,
	}
	room.Chatters[chatter] = true

	if removed := room.removeChatter(chatter); !removed {
		t.Fatal("expected chatter to be removed on first call")
	}

	if removed := room.removeChatter(chatter); removed {
		t.Fatal("expected second remove call to be no-op")
	}
}

func TestMessageWsHandler_RemovesOverloadedChatterSafely(t *testing.T) {
	t.Parallel()

	room := NewWsRoom("public")
	chatter := &Chatter{
		Uid:  42,
		Send: make(chan []byte, 1),
		Room: room,
	}
	room.Chatters[chatter] = true

	// Fill sender buffer to force default branch and removal path.
	chatter.Send <- []byte("occupied")
	MessageWsHandler([]byte(`{"u":42,"t":"x"}`), room)

	if _, exists := room.Chatters[chatter]; exists {
		t.Fatal("expected overloaded chatter to be removed from room")
	}

	// Simulate delayed Leave handling after removal.
	if removed := room.removeChatter(chatter); removed {
		t.Fatal("expected delayed remove to be no-op")
	}
}

func TestMessageWsHandler_ForwardsToMatchingUser(t *testing.T) {
	t.Parallel()

	room := NewWsRoom("public")
	target := &Chatter{
		Uid:  42,
		Send: make(chan []byte, 1),
		Room: room,
	}
	other := &Chatter{
		Uid:  77,
		Send: make(chan []byte, 1),
		Room: room,
	}
	room.Chatters[target] = true
	room.Chatters[other] = true

	msg := []byte(`{"u":42,"t":"sd"}`)
	MessageWsHandler(msg, room)

	if got := <-target.Send; !reflect.DeepEqual(got, msg) {
		t.Fatalf("target message = %s; want %s", got, msg)
	}
	if len(other.Send) != 0 {
		t.Fatal("expected other user not to receive targeted message")
	}
}

func TestMessageWsHandler_BroadcastsWhenUserIDIsZero(t *testing.T) {
	t.Parallel()

	room := NewWsRoom("public")
	first := &Chatter{Uid: 42, Send: make(chan []byte, 1), Room: room}
	second := &Chatter{Uid: 77, Send: make(chan []byte, 1), Room: room}
	room.Chatters[first] = true
	room.Chatters[second] = true

	msg := []byte(`{"u":0,"t":"system"}`)
	MessageWsHandler(msg, room)

	if got := <-first.Send; !reflect.DeepEqual(got, msg) {
		t.Fatalf("first message = %s; want %s", got, msg)
	}
	if got := <-second.Send; !reflect.DeepEqual(got, msg) {
		t.Fatalf("second message = %s; want %s", got, msg)
	}
}

func TestMessageWsHandler_IgnoresInvalidJSON(t *testing.T) {
	t.Parallel()

	room := NewWsRoom("public")
	chatter := &Chatter{Uid: 42, Send: make(chan []byte, 1), Room: room}
	room.Chatters[chatter] = true

	MessageWsHandler([]byte("{"), room)

	if len(chatter.Send) != 0 {
		t.Fatal("expected invalid message not to be forwarded")
	}
	if _, exists := room.Chatters[chatter]; !exists {
		t.Fatal("expected chatter to stay connected")
	}
}

func TestWsHandlerServeHTTP_AllowsPrivilegedClientToForwardMessage(t *testing.T) {
	room := NewWsRoom("public")
	go room.Run(MessageWsHandler)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), domain.ContextWebsocketUserIDKey, uint64(42))
		ctx = context.WithValue(ctx, domain.ContextWebsocketCanSendKey, true)
		(&WsHandler{Room: room}).ServeHTTP(w, r.WithContext(ctx))
	}))
	defer server.Close()

	wsURL := mustWSURL(t, server.URL)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"u":42,"t":"sd"}`)); err != nil {
		t.Fatalf("write message: %v", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read forwarded message: %v", err)
	}
	if string(msg) != `{"u":42,"t":"sd"}` {
		t.Fatalf("message = %s", msg)
	}
}

func mustWSURL(t testing.TB, raw string) url.URL {
	t.Helper()

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	u.Scheme = "ws"
	return *u
}
