package domain

import (
	"reflect"
	"testing"

	"github.com/whagency/core-common/testkit/loggertest"
)

func TestNewChannelFromContactField(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		value   string
		want    Channel
		wantErr bool
	}{
		{name: "email", value: "email", want: ChannelEmail},
		{name: "phone", value: "phone", want: ChannelSMS},
		{name: "user id", value: "userId", want: ChannelWebSocket},
		{name: "trimmed value", value: " email ", want: ChannelEmail},
		{name: "unsupported", value: "telegram", wantErr: true},
		{name: "empty", value: "", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewChannelFromContactField(tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("channel = %d; want %d", got, tc.want)
			}
			if got.ContactField() == "" {
				t.Fatalf("expected non-empty contact field")
			}
		})
	}
}

func TestNewOTPMessageKindFromPurpose(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		value   string
		want    MessageKind
		wantErr bool
	}{
		{name: "empty defaults to registration", value: "", want: MsgOTPRegistration},
		{name: "registration", value: "registrationOtp", want: MsgOTPRegistration},
		{name: "password reset", value: "passwordResetOtp", want: MsgOTPPasswordReset},
		{name: "two factor", value: "twoFactorOtp", want: MsgOTPTwoFactor},
		{name: "trimmed", value: " twoFactorOtp ", want: MsgOTPTwoFactor},
		{name: "unsupported", value: "loginOtp", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewOTPMessageKindFromPurpose(tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("kind = %d; want %d", got, tc.want)
			}
		})
	}
}

func TestNewPushMessageKindFromPurpose(t *testing.T) {
	t.Parallel()

	cases := []struct {
		value string
		want  MessageKind
	}{
		{value: "successfulDeposit", want: MsgPushSuccessfulDeposit},
		{value: "requiredVerificationDeposit", want: MsgPushRequiredVerificationDeposit},
		{value: "successfulWithdrawal", want: MsgPushSuccessfulWithdrawal},
		{value: "successfulIncomingTransfer", want: MsgPushSuccessfulIncomingTransfer},
		{value: "successfulOutgoingTransfer", want: MsgPushSuccessfulOutgoingTransfer},
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			t.Parallel()

			got, err := NewPushMessageKindFromPurpose(" " + tc.value + " ")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("kind = %d; want %d", got, tc.want)
			}
		})
	}

	t.Run("unsupported", func(t *testing.T) {
		t.Parallel()

		if _, err := NewPushMessageKindFromPurpose("unknown"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestGetPushChannelsByMessageKind(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		kind MessageKind
		want []Channel
	}{
		{name: "successful deposit", kind: MsgPushSuccessfulDeposit, want: []Channel{ChannelWebSocket, ChannelEmail}},
		{name: "required verification", kind: MsgPushRequiredVerificationDeposit, want: []Channel{ChannelWebSocket}},
		{name: "outgoing transfer", kind: MsgPushSuccessfulOutgoingTransfer, want: []Channel{ChannelEmail}},
		{name: "unsupported", kind: MessageKind(99), want: []Channel{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := GetPushChannelsByMessageKind(tc.kind)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("channels = %#v; want %#v", got, tc.want)
			}
		})
	}
}

func TestLocaleHelpers(t *testing.T) {
	loggertest.Init(t, "notification-service-test")

	cases := []struct {
		value string
		want  Locale
	}{
		{value: "en", want: LocaleEn},
		{value: "EN-us", want: LocaleEn},
		{value: "uk_UA", want: LocaleUk},
		{value: "fr", want: LocaleEn},
		{value: "", want: LocaleEn},
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			got := NewLocaleFromString(tc.value)
			if got != tc.want {
				t.Fatalf("locale = %q; want %q", got, tc.want)
			}
		})
	}

	if !reflect.DeepEqual(SupportedLocaleStrings(), []string{"en", "uk"}) {
		t.Fatalf("unexpected supported locale strings: %#v", SupportedLocaleStrings())
	}
	if !IsSupportedLocale(LocaleUk) {
		t.Fatal("expected uk to be supported")
	}
	if IsSupportedLocale(Locale("fr")) {
		t.Fatal("expected fr to be unsupported")
	}
}

func TestTemplateDataAccessors(t *testing.T) {
	t.Parallel()

	data := TemplateData{
		"string": "value",
		"int":    42,
		"float":  12.5,
		"nil":    nil,
	}

	if got := data.GetString("string"); got != "value" {
		t.Fatalf("string = %q", got)
	}
	if got := data.GetString("int"); got != "42" {
		t.Fatalf("int string = %q", got)
	}
	if got := data.GetString("missing"); got != "" {
		t.Fatalf("missing string = %q", got)
	}
	if got := data.GetFloat("float"); got != 12.5 {
		t.Fatalf("float = %v", got)
	}
	if got := data.GetFloat("int"); got != 0 {
		t.Fatalf("int float = %v", got)
	}
}
