package controllers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"notification-service/internal/domain"
)

// stubDedupeKeys records fence operations and lets tests drive their outcomes.
type stubDedupeKeys struct {
	mu sync.Mutex

	reserved map[string]struct{}
	released []string

	reserveErr error
	releaseErr error
}

func newStubDedupeKeys() *stubDedupeKeys {
	return &stubDedupeKeys{reserved: make(map[string]struct{})}
}

func (s *stubDedupeKeys) Reserve(_ context.Context, key string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reserveErr != nil {
		return false, s.reserveErr
	}
	if _, exists := s.reserved[key]; exists {
		return true, nil
	}
	s.reserved[key] = struct{}{}
	return false, nil
}

func (s *stubDedupeKeys) Release(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.releaseErr != nil {
		return s.releaseErr
	}
	delete(s.reserved, key)
	s.released = append(s.released, key)
	return nil
}

func (s *stubDedupeKeys) DeleteExpired(context.Context) (int64, error) { return 0, nil }

func (s *stubDedupeKeys) releaseCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.released)
}

func pushCommand(dedupeKey string) domain.SendCommand {
	return domain.SendCommand{
		Kind:      domain.MsgPushSuccessfulOutgoingTransfer,
		To:        []domain.Address{{Channel: domain.ChannelWebSocket, Value: "1"}},
		Data:      domain.TemplateData{_userIdParamKey: "1"},
		DedupeKey: dedupeKey,
	}
}

// multiChannelCommand mirrors the real fan-out of a successful deposit, which
// goes to both WebSocket and e-mail.
func multiChannelCommand(dedupeKey string) domain.SendCommand {
	return domain.SendCommand{
		Kind: domain.MsgPushSuccessfulDeposit,
		To: []domain.Address{
			{Channel: domain.ChannelWebSocket, Value: "1"},
			{Channel: domain.ChannelEmail, Value: "user@example.com"},
		},
		Data: domain.TemplateData{_userIdParamKey: "1", "email": "user@example.com"},
		// Explicit locale keeps the e-mail path off the user-service fallback,
		// which is not what these tests are about.
		Locale:    domain.LocaleEn,
		DedupeKey: dedupeKey,
	}
}

// Most push notifications fan out to several channels that fail independently.
// A fence covering the whole command would be released by the one channel that
// failed, and the producer's retry would then deliver the channel that had
// already succeeded a second time.
func TestNotificationCtrlSend_PartialChannelFailureDoesNotResendSucceededChannel(t *testing.T) {
	t.Parallel()

	mailer := &stubMailer{err: errors.New("smtp down")}
	websocket := &stubWebsocketCtrl{}
	fence := newStubDedupeKeys()
	ctrl := NewNotificationController(mailer, websocket, nil, fence, time.Hour)

	cmd := multiChannelCommand("push:deposit:99:success")

	// First attempt: websocket lands, e-mail fails, so the command reports failure.
	if err := ctrl.Send(context.Background(), cmd); err == nil {
		t.Fatal("send must report the failed e-mail channel")
	}
	if websocket.calls != 1 || mailer.calls != 1 {
		t.Fatalf("first attempt: websocket=%d mailer=%d, want 1 and 1", websocket.calls, mailer.calls)
	}

	// Producer retries the whole message. Only the failed channel may be retried.
	mailer.err = nil
	if err := ctrl.Send(context.Background(), cmd); err != nil {
		t.Fatalf("retry: %v", err)
	}

	if websocket.calls != 1 {
		t.Errorf("websocket sends = %d, want 1 (already delivered, must not repeat)", websocket.calls)
	}
	if mailer.calls != 2 {
		t.Errorf("mailer sends = %d, want 2 (failed channel must be retried)", mailer.calls)
	}
}

// Channels must not share a fence, otherwise the first one to deliver would
// suppress the others on the very same command.
func TestNotificationCtrlSend_ChannelsAreFencedIndependently(t *testing.T) {
	t.Parallel()

	mailer := &stubMailer{}
	websocket := &stubWebsocketCtrl{}
	fence := newStubDedupeKeys()
	ctrl := NewNotificationController(mailer, websocket, nil, fence, time.Hour)

	if err := ctrl.Send(context.Background(), multiChannelCommand("push:deposit:100:success")); err != nil {
		t.Fatalf("send: %v", err)
	}

	if websocket.calls != 1 {
		t.Errorf("websocket sends = %d, want 1", websocket.calls)
	}
	if mailer.calls != 1 {
		t.Errorf("mailer sends = %d, want 1 (a shared fence would have suppressed this)", mailer.calls)
	}
}

// Producers deliver at-least-once: a publish that lands before its status update
// fails is republished. The user cannot tell that apart from a real second event,
// so the second delivery must be dropped.
func TestNotificationCtrlSend_DuplicateDedupeKeyIsSkipped(t *testing.T) {
	t.Parallel()

	websocket := &stubWebsocketCtrl{}
	fence := newStubDedupeKeys()
	ctrl := NewNotificationController(&stubMailer{}, websocket, nil, fence, time.Hour)

	cmd := pushCommand("push:transfer:42:outgoing")

	if err := ctrl.Send(context.Background(), cmd); err != nil {
		t.Fatalf("first send: %v", err)
	}
	if err := ctrl.Send(context.Background(), cmd); err != nil {
		t.Fatalf("duplicate send must succeed silently: %v", err)
	}

	if websocket.calls != 1 {
		t.Errorf("websocket sends = %d, want 1 (duplicate must not reach the user)", websocket.calls)
	}
}

// A fence that is claimed but never delivered would swallow the producer's
// retry, turning a transient failure into a permanently lost notification.
func TestNotificationCtrlSend_FenceReleasedWhenDeliveryFails(t *testing.T) {
	t.Parallel()

	websocket := &stubWebsocketCtrl{err: errors.New("websocket down")}
	fence := newStubDedupeKeys()
	ctrl := NewNotificationController(&stubMailer{}, websocket, nil, fence, time.Hour)

	cmd := pushCommand("push:transfer:43:outgoing")

	if err := ctrl.Send(context.Background(), cmd); err == nil {
		t.Fatal("send must report the delivery failure")
	}
	if fence.releaseCount() != 1 {
		t.Fatalf("fence releases = %d, want 1", fence.releaseCount())
	}

	// The retry must be allowed through and must succeed once delivery recovers.
	websocket.err = nil
	if err := ctrl.Send(context.Background(), cmd); err != nil {
		t.Fatalf("retry after released fence: %v", err)
	}
	if websocket.calls != 2 {
		t.Errorf("websocket sends = %d, want 2 (failed attempt + successful retry)", websocket.calls)
	}
}

// Producers that publish exactly once send no key; fencing must stay off for
// them rather than silently collapsing genuinely distinct notifications.
func TestNotificationCtrlSend_NoDedupeKeyDeliversEveryTime(t *testing.T) {
	t.Parallel()

	websocket := &stubWebsocketCtrl{}
	fence := newStubDedupeKeys()
	ctrl := NewNotificationController(&stubMailer{}, websocket, nil, fence, time.Hour)

	cmd := pushCommand("")

	for range 3 {
		if err := ctrl.Send(context.Background(), cmd); err != nil {
			t.Fatalf("send: %v", err)
		}
	}

	if websocket.calls != 3 {
		t.Errorf("websocket sends = %d, want 3 (unkeyed sends must not be fenced)", websocket.calls)
	}
}

// If the fence itself is unavailable, delivering anyway would defeat the point.
// Failing is retryable and costs a delay; delivering unfenced costs a duplicate.
func TestNotificationCtrlSend_FenceFailureIsReportedNotBypassed(t *testing.T) {
	t.Parallel()

	websocket := &stubWebsocketCtrl{}
	fence := newStubDedupeKeys()
	fence.reserveErr = errors.New("database unavailable")
	ctrl := NewNotificationController(&stubMailer{}, websocket, nil, fence, time.Hour)

	if err := ctrl.Send(context.Background(), pushCommand("push:transfer:44:outgoing")); err == nil {
		t.Fatal("send must fail when the dedupe fence cannot be claimed")
	}
	if websocket.calls != 0 {
		t.Errorf("websocket sends = %d, want 0 (must not deliver unfenced)", websocket.calls)
	}
}

// A controller built without a fence repository must keep working — the fence is
// optional infrastructure, not a hard dependency.
func TestNotificationCtrlSend_NilRepositoryDisablesFencing(t *testing.T) {
	t.Parallel()

	websocket := &stubWebsocketCtrl{}
	ctrl := NewNotificationController(&stubMailer{}, websocket, nil, nil, time.Hour)

	cmd := pushCommand("push:transfer:45:outgoing")
	for range 2 {
		if err := ctrl.Send(context.Background(), cmd); err != nil {
			t.Fatalf("send: %v", err)
		}
	}

	if websocket.calls != 2 {
		t.Errorf("websocket sends = %d, want 2", websocket.calls)
	}
}
