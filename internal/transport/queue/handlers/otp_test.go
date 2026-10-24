package handlers

import (
	"reflect"
	"strings"
	"testing"

	"notification-service/internal/domain"
	"notification-service/test/fixtures"

	commonNotificationQueue "github.com/whagency/core-common/services/notification/queue"
)

func TestDecodeOTPMessage_Table(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		message     commonNotificationQueue.OTPMessage
		want        domain.SendCommand
		wantErrText string
	}{
		{
			name:    "registration email",
			message: fixtures.OTPMessage(),
			want: domain.SendCommand{
				Kind:   domain.MsgOTPRegistration,
				Locale: domain.LocaleEn,
				To: []domain.Address{
					{Channel: domain.ChannelEmail, Value: "user@example.test"},
				},
				Data: domain.TemplateData{"CodeDigits": []string{"1", "2", "3", "4", "5", "6"}},
			},
		},
		{
			name: "empty purpose defaults to registration and empty locale stays empty",
			message: commonNotificationQueue.OTPMessage{
				Purpose:      "",
				ContactType:  "email",
				ContactValue: "user@example.test",
				Code:         "42",
			},
			want: domain.SendCommand{
				Kind: domain.MsgOTPRegistration,
				To: []domain.Address{
					{Channel: domain.ChannelEmail, Value: "user@example.test"},
				},
				Data: domain.TemplateData{"CodeDigits": []string{"4", "2"}},
			},
		},
		{
			name: "phone is accepted by decoder",
			message: commonNotificationQueue.OTPMessage{
				Purpose:      "twoFactorOtp",
				ContactType:  "phone",
				ContactValue: "+10000000000",
				Code:         "654321",
				Locale:       "uk-UA",
			},
			want: domain.SendCommand{
				Kind:   domain.MsgOTPTwoFactor,
				Locale: domain.LocaleUk,
				To: []domain.Address{
					{Channel: domain.ChannelSMS, Value: "+10000000000"},
				},
				Data: domain.TemplateData{"CodeDigits": []string{"6", "5", "4", "3", "2", "1"}},
			},
		},
		{
			name: "websocket contact type rejected for OTP",
			message: commonNotificationQueue.OTPMessage{
				Purpose:      "registrationOtp",
				ContactType:  "userId",
				ContactValue: "42",
				Code:         "123456",
			},
			wantErrText: "unsupported otp channel",
		},
		{
			name: "unsupported contact type rejected",
			message: commonNotificationQueue.OTPMessage{
				Purpose:      "registrationOtp",
				ContactType:  "telegram",
				ContactValue: "42",
				Code:         "123456",
			},
			wantErrText: "unsupported channel contact field",
		},
		{
			name: "unsupported purpose rejected",
			message: commonNotificationQueue.OTPMessage{
				Purpose:      "loginOtp",
				ContactType:  "email",
				ContactValue: "user@example.test",
				Code:         "123456",
			},
			wantErrText: "unsupported otp purpose",
		},
		{
			name: "empty contact rejected",
			message: commonNotificationQueue.OTPMessage{
				Purpose:     "registrationOtp",
				ContactType: "email",
				Code:        "123456",
			},
			wantErrText: "empty contact value",
		},
		{
			name: "empty code rejected",
			message: commonNotificationQueue.OTPMessage{
				Purpose:      "registrationOtp",
				ContactType:  "email",
				ContactValue: "user@example.test",
			},
			wantErrText: "empty code",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := decodeOTPMessage(tc.message)
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

func TestSplitCodeDigits_KeepsRunes(t *testing.T) {
	t.Parallel()

	got := splitCodeDigits("1a🙂")
	want := []string{"1", "a", "🙂"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("digits = %#v; want %#v", got, want)
	}
}
