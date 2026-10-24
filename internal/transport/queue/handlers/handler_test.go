package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"notification-service/internal/domain"
	svcErrors "notification-service/internal/domain/errors"

	appErrors "github.com/whagency/core-common/errorsx"
	"github.com/whagency/core-common/queue"
	commonNotificationQueue "github.com/whagency/core-common/services/notification/queue"
	"github.com/whagency/core-common/testkit/loggertest"
)

type fakeNotificationController struct {
	err  error
	sent []domain.SendCommand
}

func (f *fakeNotificationController) Send(_ context.Context, cmd domain.SendCommand) error {
	f.sent = append(f.sent, cmd)
	return f.err
}

func TestOTPHandlerHandle(t *testing.T) {
	loggertest.Init(t, "notification-service")

	validBody := mustJSON(t, commonNotificationQueue.OTPMessage{
		Purpose:      "registrationOtp",
		ContactType:  "email",
		ContactValue: "user@example.test",
		Code:         "123456",
		Locale:       "en",
	})

	cases := []struct {
		name            string
		msgType         queue.MessageType
		body            []byte
		ctrlErr         error
		wantNonRetry    bool
		wantRetryable   bool
		wantSendInvoked bool
	}{
		{
			name:         "unexpected message type is non-retryable",
			msgType:      commonNotificationQueue.MessageTypePush,
			body:         validBody,
			wantNonRetry: true,
		},
		{
			name:         "bad JSON is non-retryable",
			msgType:      commonNotificationQueue.MessageTypeOTP,
			body:         []byte("{"),
			wantNonRetry: true,
		},
		{
			name: "invalid payload is non-retryable",
			body: mustJSON(t, commonNotificationQueue.OTPMessage{
				Purpose:      "registrationOtp",
				ContactType:  "email",
				ContactValue: "",
				Code:         "123456",
			}),
			wantNonRetry: true,
		},
		{
			name:            "permanent controller error is non-retryable",
			msgType:         commonNotificationQueue.MessageTypeOTP,
			body:            validBody,
			ctrlErr:         svcErrors.New(appErrors.RecordNotFoundError),
			wantNonRetry:    true,
			wantSendInvoked: true,
		},
		{
			name:            "transient controller error stays retryable",
			msgType:         commonNotificationQueue.MessageTypeOTP,
			body:            validBody,
			ctrlErr:         errors.New("smtp unavailable"),
			wantRetryable:   true,
			wantSendInvoked: true,
		},
		{
			name:            "valid message is sent",
			msgType:         commonNotificationQueue.MessageTypeOTP,
			body:            validBody,
			wantSendInvoked: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := &fakeNotificationController{err: tc.ctrlErr}
			err := NewOTPHandler(ctrl).Handle(context.Background(), tc.msgType, tc.body)

			assertRetryExpectation(t, err, tc.wantNonRetry, tc.wantRetryable)
			if tc.wantSendInvoked && len(ctrl.sent) != 1 {
				t.Fatalf("Send calls = %d; want 1", len(ctrl.sent))
			}
			if !tc.wantSendInvoked && len(ctrl.sent) != 0 {
				t.Fatalf("Send calls = %d; want 0", len(ctrl.sent))
			}
		})
	}
}

func TestPushHandlerHandle(t *testing.T) {
	loggertest.Init(t, "notification-service")

	validBody := mustJSON(t, commonNotificationQueue.PushMessage{
		Purpose: "successfulDeposit",
		Data: map[string]any{
			"userId": "42",
			"email":  "user@example.test",
		},
		Locale: "uk",
	})

	cases := []struct {
		name            string
		msgType         queue.MessageType
		body            []byte
		ctrlErr         error
		wantNonRetry    bool
		wantRetryable   bool
		wantSendInvoked bool
	}{
		{
			name:         "unexpected message type is non-retryable",
			msgType:      commonNotificationQueue.MessageTypeOTP,
			body:         validBody,
			wantNonRetry: true,
		},
		{
			name:         "bad JSON is non-retryable",
			msgType:      commonNotificationQueue.MessageTypePush,
			body:         []byte("{"),
			wantNonRetry: true,
		},
		{
			name: "invalid payload is non-retryable",
			body: mustJSON(t, commonNotificationQueue.PushMessage{
				Purpose: "successfulDeposit",
				Data: map[string]any{
					"userId": "abc",
				},
			}),
			wantNonRetry: true,
		},
		{
			name:            "permanent controller error is non-retryable",
			msgType:         commonNotificationQueue.MessageTypePush,
			body:            validBody,
			ctrlErr:         svcErrors.New(appErrors.InvalidRequestError),
			wantNonRetry:    true,
			wantSendInvoked: true,
		},
		{
			name:            "transient controller error stays retryable",
			msgType:         commonNotificationQueue.MessageTypePush,
			body:            validBody,
			ctrlErr:         errors.New("websocket loopback unavailable"),
			wantRetryable:   true,
			wantSendInvoked: true,
		},
		{
			name:            "valid message is sent",
			msgType:         commonNotificationQueue.MessageTypePush,
			body:            validBody,
			wantSendInvoked: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := &fakeNotificationController{err: tc.ctrlErr}
			err := NewPushHandler(ctrl).Handle(context.Background(), tc.msgType, tc.body)

			assertRetryExpectation(t, err, tc.wantNonRetry, tc.wantRetryable)
			if tc.wantSendInvoked && len(ctrl.sent) != 1 {
				t.Fatalf("Send calls = %d; want 1", len(ctrl.sent))
			}
			if !tc.wantSendInvoked && len(ctrl.sent) != 0 {
				t.Fatalf("Send calls = %d; want 0", len(ctrl.sent))
			}
		})
	}
}

func assertRetryExpectation(t *testing.T, err error, wantNonRetry, wantRetryable bool) {
	t.Helper()

	if !wantNonRetry && !wantRetryable {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("expected error")
	}
	if wantNonRetry && !errors.Is(err, queue.NonRetryableErr) {
		t.Fatalf("expected non-retryable error, got %v", err)
	}
	if wantRetryable && errors.Is(err, queue.NonRetryableErr) {
		t.Fatalf("expected retryable error, got %v", err)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return data
}
