package controllers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"notification-service/internal/domain"
	svcErrors "notification-service/internal/domain/errors"
	testmocks "notification-service/test/mocks"

	"github.com/stretchr/testify/mock"
	appErrors "github.com/whagency/core-common/errorsx"
	"github.com/whagency/core-common/services/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNotificationCtrlSend_EmailFallbacks(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		cmd           domain.SendCommand
		profile       user.Profile
		profileErr    error
		wantMail      *domain.MailMessage
		wantErrText   string
		wantUserCalls int
	}{
		{
			name: "defaults locale when email is present and user id missing",
			cmd: domain.SendCommand{
				Kind: domain.MsgPushSuccessfulOutgoingTransfer,
				To: []domain.Address{
					{Channel: domain.ChannelEmail, Value: "jane@example.test"},
				},
				Data: domain.TemplateData{},
			},
			wantMail: &domain.MailMessage{
				Kind:   domain.MsgPushSuccessfulOutgoingTransfer,
				To:     "jane@example.test",
				Params: domain.TemplateData{},
				Locale: domain.LocaleEn,
			},
		},
		{
			name: "uses user profile email and locale when address is empty",
			cmd: domain.SendCommand{
				Kind: domain.MsgPushSuccessfulOutgoingTransfer,
				To: []domain.Address{
					{Channel: domain.ChannelEmail, Value: ""},
				},
				Data: domain.TemplateData{"userId": "42"},
			},
			profile: user.Profile{
				ContactType:  user.ContactTypeEmail,
				ContactValue: "profile@example.test",
				Locale:       "uk-UA",
			},
			wantMail: &domain.MailMessage{
				Kind:   domain.MsgPushSuccessfulOutgoingTransfer,
				To:     "profile@example.test",
				Params: domain.TemplateData{"userId": "42"},
				Locale: domain.LocaleUk,
			},
			wantUserCalls: 1,
		},
		{
			name: "uses user profile locale when address is present but locale is empty",
			cmd: domain.SendCommand{
				Kind: domain.MsgPushSuccessfulDeposit,
				To: []domain.Address{
					{Channel: domain.ChannelEmail, Value: "jane@example.test"},
				},
				Data: domain.TemplateData{"userId": "7"},
			},
			profile: user.Profile{
				ContactType:  user.ContactTypeEmail,
				ContactValue: "profile@example.test",
				Locale:       "uk",
			},
			wantMail: &domain.MailMessage{
				Kind:   domain.MsgPushSuccessfulDeposit,
				To:     "jane@example.test",
				Params: domain.TemplateData{"userId": "7"},
				Locale: domain.LocaleUk,
			},
			wantUserCalls: 1,
		},
		{
			name: "rejects empty email without user id",
			cmd: domain.SendCommand{
				Kind: domain.MsgPushSuccessfulOutgoingTransfer,
				To: []domain.Address{
					{Channel: domain.ChannelEmail, Value: ""},
				},
				Data: domain.TemplateData{},
			},
			wantErrText: "empty userId",
		},
		{
			name: "rejects phone profile for email fallback",
			cmd: domain.SendCommand{
				Kind: domain.MsgPushSuccessfulOutgoingTransfer,
				To: []domain.Address{
					{Channel: domain.ChannelEmail, Value: ""},
				},
				Data: domain.TemplateData{"userId": "42"},
			},
			profile: user.Profile{
				ContactType:  user.ContactTypePhone,
				ContactValue: "+1000000000",
				Locale:       "en",
			},
			wantErrText:   "contact type is not email",
			wantUserCalls: 1,
		},
		{
			name: "rejects invalid user id",
			cmd: domain.SendCommand{
				Kind: domain.MsgPushSuccessfulOutgoingTransfer,
				To: []domain.Address{
					{Channel: domain.ChannelEmail, Value: ""},
				},
				Data: domain.TemplateData{"userId": "abc"},
			},
			wantErrText: "invalid userId",
		},
		{
			name: "maps profile not found into not found error",
			cmd: domain.SendCommand{
				Kind: domain.MsgPushSuccessfulOutgoingTransfer,
				To: []domain.Address{
					{Channel: domain.ChannelEmail, Value: ""},
				},
				Data: domain.TemplateData{"userId": "42"},
			},
			profileErr:    status.Error(codes.NotFound, "profile missing"),
			wantErrText:   "profile missing",
			wantUserCalls: 1,
		},
		{
			name: "keeps transient profile errors retryable",
			cmd: domain.SendCommand{
				Kind: domain.MsgPushSuccessfulOutgoingTransfer,
				To: []domain.Address{
					{Channel: domain.ChannelEmail, Value: ""},
				},
				Data: domain.TemplateData{"userId": "42"},
			},
			profileErr:    status.Error(codes.Unavailable, "user grpc down"),
			wantErrText:   "user grpc down",
			wantUserCalls: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mailer := testmocks.NewMockMailerManager(t)
			websocket := testmocks.NewMockWebsocketController(t)
			userGRPC := testmocks.NewMockUserServiceGRPC(t)

			if tc.wantMail != nil {
				mailer.EXPECT().
					Send(mock.Anything, *tc.wantMail).
					Return(nil)
			}
			if tc.wantUserCalls > 0 {
				userGRPC.EXPECT().
					GetProfile(mock.Anything, mock.Anything).
					Return(tc.profile, tc.profileErr).
					Times(tc.wantUserCalls)
			}

			ctrl := NewNotificationController(mailer, websocket, userGRPC, nil, 0)
			err := ctrl.Send(context.Background(), tc.cmd)
			if tc.wantErrText == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErrText) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErrText, err)
			}
		})
	}
}

func TestNotificationCtrlSend_WebsocketRecipientValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		to          string
		wantMessage *domain.WsMessage
		wantErrText string
	}{
		{
			name: "valid recipient",
			to:   "42",
			wantMessage: &domain.WsMessage{
				Kind:   domain.MsgPushSuccessfulDeposit,
				To:     42,
				Params: domain.TemplateData{"asset": "USDT", "amount": 12.5},
				Locale: domain.LocaleEn,
			},
		},
		{name: "invalid recipient", to: "abc", wantErrText: "invalid websocket recipient id"},
		{name: "empty recipient", to: "", wantErrText: "invalid websocket recipient id"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mailer := testmocks.NewMockMailerManager(t)
			websocket := testmocks.NewMockWebsocketController(t)
			if tc.wantMessage != nil {
				websocket.EXPECT().
					Send(mock.Anything, *tc.wantMessage).
					Return(nil)
			}

			ctrl := NewNotificationController(mailer, websocket, nil, nil, 0)
			err := ctrl.Send(context.Background(), domain.SendCommand{
				Kind: domain.MsgPushSuccessfulDeposit,
				To: []domain.Address{
					{Channel: domain.ChannelWebSocket, Value: tc.to},
				},
				Data:   domain.TemplateData{"asset": "USDT", "amount": 12.5},
				Locale: domain.LocaleEn,
			})
			if tc.wantErrText == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErrText) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErrText, err)
			}
		})
	}
}

func TestNotificationCtrlSend_ErrorClassificationInputs(t *testing.T) {
	t.Parallel()

	mailer := testmocks.NewMockMailerManager(t)
	websocket := testmocks.NewMockWebsocketController(t)
	mailer.EXPECT().
		Send(mock.Anything, mock.Anything).
		Return(svcErrors.New(appErrors.RecordNotFoundError))
	websocket.EXPECT().
		Send(mock.Anything, mock.Anything).
		Return(errors.New("websocket unavailable"))

	ctrl := NewNotificationController(mailer, websocket, nil, nil, 0)
	err := ctrl.Send(context.Background(), domain.SendCommand{
		Kind: domain.MsgPushSuccessfulDeposit,
		To: []domain.Address{
			{Channel: domain.ChannelEmail, Value: "jane@example.test"},
			{Channel: domain.ChannelWebSocket, Value: "42"},
		},
		Data:   domain.TemplateData{"asset": "USDT", "amount": 12.5},
		Locale: domain.LocaleEn,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "websocket unavailable") {
		t.Fatalf("expected websocket cause, got %v", err)
	}
	if !strings.Contains(err.Error(), "record not found") {
		t.Fatalf("expected joined error to preserve service error, got %v", err)
	}
}
