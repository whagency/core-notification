package handlers

import (
	"reflect"
	"strings"
	"testing"

	"notification-service/internal/domain"
	"notification-service/test/fixtures"

	commonNotificationQueue "github.com/whagency/core-common/services/notification/queue"
)

func TestDecodePushMessage(t *testing.T) {
	t.Run("rejects non numeric websocket user id", func(t *testing.T) {
		_, err := decodePushMessage(commonNotificationQueue.PushMessage{
			Purpose: "successfulDeposit",
			Data: map[string]any{
				"userId": "abc",
			},
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rejects email fallback without user id", func(t *testing.T) {
		_, err := decodePushMessage(commonNotificationQueue.PushMessage{
			Purpose: "successfulOutgoingTransfer",
			Data:    map[string]any{},
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("accepts numeric user id for websocket and email fallback", func(t *testing.T) {
		cmd, err := decodePushMessage(commonNotificationQueue.PushMessage{
			Purpose: "successfulDeposit",
			Data: map[string]any{
				"userId": "42",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cmd.To) != 2 {
			t.Fatalf("unexpected addresses count: got %d", len(cmd.To))
		}
	})
}

func TestDecodePushMessage_Table(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		message     commonNotificationQueue.PushMessage
		want        domain.SendCommand
		wantErrText string
	}{
		{
			name:    "successful deposit uses websocket and email",
			message: fixtures.PushMessage("successfulDeposit"),
			want: domain.SendCommand{
				Kind:   domain.MsgPushSuccessfulDeposit,
				Locale: domain.LocaleEn,
				To: []domain.Address{
					{Channel: domain.ChannelWebSocket, Value: "42"},
					{Channel: domain.ChannelEmail, Value: "user@example.test"},
				},
				Data: domain.TemplateData{
					"userId": "42",
					"email":  "user@example.test",
					"asset":  "USDT",
					"amount": 12.5,
				},
			},
		},
		{
			name: "required verification deposit is websocket only",
			message: commonNotificationQueue.PushMessage{
				Purpose: "requiredVerificationDeposit",
				Data: map[string]any{
					"userId": "77",
					"email":  "ignored@example.test",
				},
				Locale: "uk-UA",
			},
			want: domain.SendCommand{
				Kind:   domain.MsgPushRequiredVerificationDeposit,
				Locale: domain.LocaleUk,
				To: []domain.Address{
					{Channel: domain.ChannelWebSocket, Value: "77"},
				},
				Data: domain.TemplateData{
					"userId": "77",
					"email":  "ignored@example.test",
				},
			},
		},
		{
			name: "outgoing transfer allows email fallback through user id",
			message: commonNotificationQueue.PushMessage{
				Purpose: "successfulOutgoingTransfer",
				Data: map[string]any{
					"userId": "42",
				},
			},
			want: domain.SendCommand{
				Kind: domain.MsgPushSuccessfulOutgoingTransfer,
				To: []domain.Address{
					{Channel: domain.ChannelEmail, Value: ""},
				},
				Data: domain.TemplateData{
					"userId": "42",
				},
			},
		},
		{
			name: "unsupported purpose rejected",
			message: commonNotificationQueue.PushMessage{
				Purpose: "unknown",
				Data:    map[string]any{"userId": "42"},
			},
			wantErrText: "unsupported push purpose",
		},
		{
			name: "nil data rejected",
			message: commonNotificationQueue.PushMessage{
				Purpose: "successfulDeposit",
				Data:    nil,
			},
			wantErrText: "empty data",
		},
		{
			name: "missing websocket user id rejected",
			message: commonNotificationQueue.PushMessage{
				Purpose: "successfulDeposit",
				Data:    map[string]any{"email": "user@example.test"},
			},
			wantErrText: "empty userId value",
		},
		{
			name: "empty email fallback without user id rejected",
			message: commonNotificationQueue.PushMessage{
				Purpose: "successfulOutgoingTransfer",
				Data:    map[string]any{"email": ""},
			},
			wantErrText: "empty userId value",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := decodePushMessage(tc.message)
			if tc.wantErrText != "" {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tc.wantErrText) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErrText, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("command = %#v; want %#v", got, tc.want)
			}
		})
	}
}

func TestDecodeTemplateString(t *testing.T) {
	t.Parallel()

	data := domain.TemplateData{
		"string": " value ",
		"int":    42,
		"nil":    nil,
	}

	if got := decodeTemplateString(data, "string"); got != "value" {
		t.Fatalf("string = %q", got)
	}
	if got := decodeTemplateString(data, "int"); got != "42" {
		t.Fatalf("int = %q", got)
	}
	if got := decodeTemplateString(data, "nil"); got != "" {
		t.Fatalf("nil = %q", got)
	}
	if got := decodeTemplateString(data, "missing"); got != "" {
		t.Fatalf("missing = %q", got)
	}
}

// The dedupe key has to survive decoding, otherwise the fence downstream never
// engages and every producer retry reaches the user again.
func TestDecodePushMessage_CarriesDedupeKey(t *testing.T) {
	t.Parallel()

	cmd, err := decodePushMessage(commonNotificationQueue.PushMessage{
		Purpose:   "successfulOutgoingTransfer",
		Data:      map[string]any{"userId": "42", "amount": "1", "asset": "USDT", "toUserNumber": "7"},
		Locale:    "en",
		DedupeKey: "  push:transfer:42:outgoing  ",
	})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if cmd.DedupeKey != "push:transfer:42:outgoing" {
		t.Errorf("DedupeKey = %q, want the trimmed key", cmd.DedupeKey)
	}
}

// An absent key must stay absent rather than becoming a shared empty fence.
func TestDecodePushMessage_WithoutDedupeKeyLeavesItEmpty(t *testing.T) {
	t.Parallel()

	cmd, err := decodePushMessage(commonNotificationQueue.PushMessage{
		Purpose: "successfulOutgoingTransfer",
		Data:    map[string]any{"userId": "42", "amount": "1", "asset": "USDT", "toUserNumber": "7"},
		Locale:  "en",
	})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if cmd.DedupeKey != "" {
		t.Errorf("DedupeKey = %q, want empty", cmd.DedupeKey)
	}
}
