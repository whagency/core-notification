package controllers

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"notification-service/internal/domain"
	svcErrors "notification-service/internal/domain/errors"
	testmocks "notification-service/test/mocks"

	"github.com/stretchr/testify/mock"
	appErrors "github.com/whagency/core-common/errorsx"
)

func TestWebsocketCtrlSend(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		kind        domain.MessageKind
		wantPayload any
		wantErrText string
	}{
		{
			name: "successful deposit",
			kind: domain.MsgPushSuccessfulDeposit,
			wantPayload: domain.WsSuccessfulDepositMessage{
				Type:      "sd",
				UserID:    42,
				AssetCode: "USDT",
				Amount:    12.5,
			},
		},
		{
			name: "required verification deposit",
			kind: domain.MsgPushRequiredVerificationDeposit,
			wantPayload: domain.WsRequiredVerificationDepositMessage{
				Type:      "rvd",
				UserID:    42,
				AssetCode: "USDT",
				Amount:    12.5,
			},
		},
		{
			name: "successful withdrawal",
			kind: domain.MsgPushSuccessfulWithdrawal,
			wantPayload: domain.WsSuccessfulWithdrawalMessage{
				Type:      "sw",
				UserID:    42,
				AssetCode: "USDT",
				Amount:    12.5,
			},
		},
		{
			name: "successful incoming transfer",
			kind: domain.MsgPushSuccessfulIncomingTransfer,
			wantPayload: domain.WsSuccessfulIncomingTransfer{
				Type:      "st",
				UserID:    42,
				AssetCode: "USDT",
				Amount:    12.5,
			},
		},
		{
			name:        "outgoing transfer is not websocket message",
			kind:        domain.MsgPushSuccessfulOutgoingTransfer,
			wantErrText: "unsupported ws message kind",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			manager := testmocks.NewMockWebsocketManager(t)
			if tc.wantPayload != nil {
				manager.EXPECT().
					Send(mock.Anything, tc.wantPayload).
					Return(nil)
			}

			ctrl := NewWebsocketController(manager)
			err := ctrl.Send(context.Background(), domain.WsMessage{
				Kind: tc.kind,
				To:   42,
				Params: domain.TemplateData{
					"asset":  "USDT",
					"amount": 12.5,
				},
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
			if tc.kind == domain.MsgPushSuccessfulOutgoingTransfer {
				er, ok := appErrors.AsErrorResponse(err)
				if !ok {
					t.Fatalf("expected ErrorResponse, got %T", err)
				}
				if er.Code != svcErrors.UnsupportedNotificationKindError {
					t.Fatalf("error code = %d; want %d", er.Code, svcErrors.UnsupportedNotificationKindError)
				}
				if !strings.Contains(er.Cause.Error(), "[WebsocketCtrl:Send]") {
					t.Fatalf("expected cause to include controller prefix, got %v", er.Cause)
				}
			}
		})
	}
}

func TestWebsocketCtrlSend_ManagerError(t *testing.T) {
	t.Parallel()

	manager := testmocks.NewMockWebsocketManager(t)
	manager.EXPECT().
		Send(mock.Anything, mock.Anything).
		Return(errors.New("loopback down"))

	ctrl := NewWebsocketController(manager)
	err := ctrl.Send(context.Background(), domain.WsMessage{
		Kind:   domain.MsgPushSuccessfulDeposit,
		To:     42,
		Params: domain.TemplateData{"asset": "USDT", "amount": 12.5},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "loopback down") {
		t.Fatalf("expected loopback cause, got %v", err)
	}
}

func TestDecodeMessageObject_UsesZeroValuesForMissingTemplateData(t *testing.T) {
	t.Parallel()

	got, err := decodeMessageObject(domain.WsMessage{
		Kind:   domain.MsgPushSuccessfulDeposit,
		To:     7,
		Params: domain.TemplateData{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := domain.WsSuccessfulDepositMessage{Type: "sd", UserID: 7}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("payload = %#v; want %#v", got, want)
	}
}
