package controllers

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"notification-service/internal/domain"

	"github.com/whagency/core-common/logger"
	"github.com/whagency/core-common/services/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubMailer struct {
	err   error
	calls int
}

func (s *stubMailer) Send(_ context.Context, _ domain.MailMessage) error {
	s.calls++
	return s.err
}

type stubWebsocketCtrl struct {
	err   error
	calls int
}

func (s *stubWebsocketCtrl) Send(_ context.Context, _ domain.WsMessage) error {
	s.calls++
	return s.err
}

type stubUserGRPC struct {
	profile user.Profile
	err     error
}

func (s *stubUserGRPC) GetConnection() (*grpc.ClientConn, error) { return nil, nil }
func (s *stubUserGRPC) Reset() error                             { return nil }
func (s *stubUserGRPC) Close() error                             { return nil }
func (s *stubUserGRPC) ValidateSession(context.Context, string, bool) (user.Session, error) {
	return user.Session{}, nil
}
func (s *stubUserGRPC) GetProfile(context.Context, uint64) (user.Profile, error) {
	return s.profile, s.err
}

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

func TestNotificationCtrlSend_ReturnsNilWhenAllChannelsSucceed(t *testing.T) {
	t.Parallel()

	mailer := &stubMailer{}
	websocket := &stubWebsocketCtrl{}
	ctrl := NewNotificationController(mailer, websocket, nil, nil, 0)

	err := ctrl.Send(context.Background(), domain.SendCommand{
		Kind: domain.MsgPushSuccessfulDeposit,
		To: []domain.Address{
			{Channel: domain.ChannelEmail, Value: "jane@example.com"},
			{Channel: domain.ChannelWebSocket, Value: "42"},
		},
		Data: domain.TemplateData{
			"asset":  "BTC",
			"amount": 1.25,
		},
		Locale: domain.LocaleEn,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if mailer.calls != 1 {
		t.Fatalf("expected mailer to be called once, got %d", mailer.calls)
	}
	if websocket.calls != 1 {
		t.Fatalf("expected websocket to be called once, got %d", websocket.calls)
	}
}

func TestNotificationCtrlSend_ReturnsErrorAndContinuesToNextChannel(t *testing.T) {
	t.Parallel()

	mailer := &stubMailer{err: errors.New("smtp unavailable")}
	websocket := &stubWebsocketCtrl{}
	ctrl := NewNotificationController(mailer, websocket, nil, nil, 0)

	err := ctrl.Send(context.Background(), domain.SendCommand{
		Kind: domain.MsgPushSuccessfulDeposit,
		To: []domain.Address{
			{Channel: domain.ChannelEmail, Value: "jane@example.com"},
			{Channel: domain.ChannelWebSocket, Value: "7"},
		},
		Data: domain.TemplateData{
			"asset":  "USDT",
			"amount": 10.0,
		},
		Locale: domain.LocaleEn,
	})
	if err == nil {
		t.Fatal("expected aggregated error, got nil")
	}
	if !strings.Contains(err.Error(), "smtp unavailable") {
		t.Fatalf("expected error to contain SMTP cause, got %v", err)
	}
	if mailer.calls != 1 {
		t.Fatalf("expected mailer to be called once, got %d", mailer.calls)
	}
	if websocket.calls != 1 {
		t.Fatalf("expected websocket to still be called once, got %d", websocket.calls)
	}
}

func TestNotificationCtrlSend_ReturnsErrorForUnsupportedChannel(t *testing.T) {
	t.Parallel()

	mailer := &stubMailer{}
	websocket := &stubWebsocketCtrl{}
	ctrl := NewNotificationController(mailer, websocket, nil, nil, 0)

	err := ctrl.Send(context.Background(), domain.SendCommand{
		Kind: domain.MsgOTPRegistration,
		To: []domain.Address{
			{Channel: domain.ChannelSMS, Value: "+1000000000"},
		},
		Data: domain.TemplateData{
			"CodeDigits": []string{"1", "2", "3", "4"},
		},
		Locale: domain.LocaleEn,
	})
	if err == nil {
		t.Fatal("expected unsupported channel error, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported channel") {
		t.Fatalf("expected unsupported channel error, got %v", err)
	}
	if mailer.calls != 0 {
		t.Fatalf("expected mailer to not be called, got %d", mailer.calls)
	}
	if websocket.calls != 0 {
		t.Fatalf("expected websocket to not be called, got %d", websocket.calls)
	}
}

func TestNotificationCtrlSend_AggregatedErrorHasSinglePrefix(t *testing.T) {
	t.Parallel()

	mailer := &stubMailer{err: errors.New("smtp unavailable")}
	websocket := &stubWebsocketCtrl{err: errors.New("ws unavailable")}
	ctrl := NewNotificationController(mailer, websocket, nil, nil, 0)

	err := ctrl.Send(context.Background(), domain.SendCommand{
		Kind: domain.MsgPushSuccessfulDeposit,
		To: []domain.Address{
			{Channel: domain.ChannelEmail, Value: "jane@example.com"},
			{Channel: domain.ChannelWebSocket, Value: "123"},
		},
		Data: domain.TemplateData{
			"asset":  "BTC",
			"amount": 1.0,
		},
		Locale: domain.LocaleEn,
	})
	if err == nil {
		t.Fatal("expected aggregated error, got nil")
	}

	if strings.Count(err.Error(), "[NotificationCtrl:Send]") != 1 {
		t.Fatalf("expected single NotificationCtrl prefix in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "smtp unavailable") {
		t.Fatalf("expected smtp cause in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "ws unavailable") {
		t.Fatalf("expected websocket cause in error, got %v", err)
	}
}

func TestNotificationCtrlSend_EmailFallbackUserNotFoundReturnsTypedError(t *testing.T) {
	t.Parallel()

	ctrl := NewNotificationController(
		&stubMailer{},
		&stubWebsocketCtrl{},
		&stubUserGRPC{err: status.Error(codes.NotFound, "user not found")},
		nil,
		0,
	)

	err := ctrl.Send(context.Background(), domain.SendCommand{
		Kind: domain.MsgPushSuccessfulOutgoingTransfer,
		To: []domain.Address{
			{Channel: domain.ChannelEmail, Value: ""},
		},
		Data: domain.TemplateData{
			"userId": "42",
		},
		Locale: domain.LocaleEn,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "user not found") {
		t.Fatalf("expected user not found in error, got %v", err)
	}
}
