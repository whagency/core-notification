package mailer

import (
	"context"
	"strings"
	"testing"
	"time"

	"notification-service/internal/domain"
	svcErrors "notification-service/internal/domain/errors"
	"notification-service/test/bootstrap"
	"notification-service/tools"

	appErrors "github.com/whagency/core-common/errorsx"
)

func TestDecodeMessageTemplate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		kind domain.MessageKind
		want domain.MailMessageTemplate
	}{
		{name: "registration", kind: domain.MsgOTPRegistration, want: domain.MailMessageTemplateSignupConfirm},
		{name: "password reset", kind: domain.MsgOTPPasswordReset, want: domain.MailMessageTemplateTwoFactorConfirm},
		{name: "two factor", kind: domain.MsgOTPTwoFactor, want: domain.MailMessageTemplateTwoFactorConfirm},
		{name: "deposit", kind: domain.MsgPushSuccessfulDeposit, want: domain.MailMessageTemplateSuccessfulDeposit},
		{name: "withdrawal", kind: domain.MsgPushSuccessfulWithdrawal, want: domain.MailMessageTemplateSuccessfulWithdrawal},
		{name: "outgoing transfer", kind: domain.MsgPushSuccessfulOutgoingTransfer, want: domain.MailMessageTemplateSuccessfulOutgoingTransfer},
		{name: "incoming transfer", kind: domain.MsgPushSuccessfulIncomingTransfer, want: domain.MailMessageTemplateSuccessfulIncomingTransfer},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := decodeMessageTemplate(tc.kind)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("template = %d; want %d", got, tc.want)
			}
		})
	}

	t.Run("unsupported", func(t *testing.T) {
		t.Parallel()

		if _, err := decodeMessageTemplate(domain.MessageKind(99)); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestSenderPrepareMessage_RendersAllSupportedTemplates(t *testing.T) {
	bootstrap.InitLogger(t)

	sender := newTestSender(t, domain.MailMessageFrom{
		FromName:    "app",
		FromAddress: "no-reply@example.test",
	})

	cases := []struct {
		name     string
		kind     domain.MessageKind
		params   domain.TemplateData
		contains []string
	}{
		{
			name: "registration",
			kind: domain.MsgOTPRegistration,
			params: domain.TemplateData{
				"CodeDigits": []string{"1", "2", "3", "4", "5", "6"},
			},
			contains: []string{"Thank you for registering", "1", "6"},
		},
		{
			name: "password reset",
			kind: domain.MsgOTPPasswordReset,
			params: domain.TemplateData{
				"CodeDigits": []string{"6", "5", "4", "3", "2", "1"},
			},
			contains: []string{"two-factor authentication code", "6", "1"},
		},
		{
			name:     "deposit",
			kind:     domain.MsgPushSuccessfulDeposit,
			params:   domain.TemplateData{"asset": "USDT", "amount": 12.50000000},
			contains: []string{"deposit has been credited", "12.5 USDT"},
		},
		{
			name:     "withdrawal",
			kind:     domain.MsgPushSuccessfulWithdrawal,
			params:   domain.TemplateData{"asset": "BTC", "amount": 0.25000000},
			contains: []string{"withdraw funds", "0.25 BTC"},
		},
		{
			name: "outgoing transfer",
			kind: domain.MsgPushSuccessfulOutgoingTransfer,
			params: domain.TemplateData{
				"asset":        "USDT",
				"amount":       99.00000000,
				"toUserNumber": "100200",
			},
			contains: []string{"Transfer of USDT successful", "user 100200"},
		},
		{
			name:     "incoming transfer",
			kind:     domain.MsgPushSuccessfulIncomingTransfer,
			params:   domain.TemplateData{"asset": "ETH", "amount": 1.00000000},
			contains: []string{"incoming transfer", "1 ETH"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message := domain.MailMessage{
				Kind:   tc.kind,
				To:     "user@example.test",
				Params: tc.params,
				Locale: domain.LocaleEn,
			}
			template, errTemplate := decodeMessageTemplate(message.Kind)
			if errTemplate != nil {
				t.Fatalf("decodeMessageTemplate returned error: %v", errTemplate)
			}
			html, errRender := sender.getMailTemplate(template.Path(), message)
			if errRender != nil {
				t.Fatalf("getMailTemplate returned error: %v", errRender)
			}

			for _, needle := range tc.contains {
				if !strings.Contains(html, needle) {
					t.Fatalf("expected rendered message to contain %q", needle)
				}
			}
		})
	}
}

func TestSenderSend_LogMailerDoesNotRequireSMTPClient(t *testing.T) {
	bootstrap.InitLogger(t)

	sender := newTestSender(t, domain.MailMessageFrom{
		FromName:    "app",
		FromAddress: "no-reply@example.test",
	})

	err := sender.Send(context.Background(), domain.MailMessage{
		Kind: domain.MsgPushSuccessfulDeposit,
		To:   "user@example.test",
		Params: domain.TemplateData{
			"asset":  "USDT",
			"amount": 12.5,
		},
		Locale: domain.LocaleEn,
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
}

func TestSenderSend_PreservesErrorResponseCodeAndWrapsCause(t *testing.T) {
	bootstrap.InitLogger(t)

	sender := newTestSender(t, domain.MailMessageFrom{
		FromName:    "app",
		FromAddress: "no-reply@example.test",
	})

	err := sender.Send(context.Background(), domain.MailMessage{
		Kind:   domain.MessageKind(99),
		To:     "user@example.test",
		Locale: domain.LocaleEn,
	})
	if err == nil {
		t.Fatal("expected error")
	}

	er, ok := appErrors.AsErrorResponse(err)
	if !ok {
		t.Fatalf("expected ErrorResponse, got %T", err)
	}
	if er.Code != svcErrors.UnsupportedNotificationKindError {
		t.Fatalf("error code = %d; want %d", er.Code, svcErrors.UnsupportedNotificationKindError)
	}
	if !strings.Contains(er.Cause.Error(), "[mailer][Sender:Send]") {
		t.Fatalf("expected cause to include sender prefix, got %v", er.Cause)
	}
}

func TestSenderPrepareMessage_ReturnsValidationErrors(t *testing.T) {
	bootstrap.InitLogger(t)

	cases := []struct {
		name        string
		from        domain.MailMessageFrom
		message     domain.MailMessage
		wantErrText string
	}{
		{
			name: "invalid from",
			from: domain.MailMessageFrom{
				FromName:    "app",
				FromAddress: "not an email",
			},
			message: domain.MailMessage{
				Kind: domain.MsgPushSuccessfulDeposit,
				To:   "user@example.test",
				Params: domain.TemplateData{
					"asset":  "USDT",
					"amount": 12.5,
				},
				Locale: domain.LocaleEn,
			},
			wantErrText: "mail address",
		},
		{
			name: "invalid recipient",
			from: domain.MailMessageFrom{
				FromName:    "app",
				FromAddress: "no-reply@example.test",
			},
			message: domain.MailMessage{
				Kind: domain.MsgPushSuccessfulDeposit,
				To:   "not an email",
				Params: domain.TemplateData{
					"asset":  "USDT",
					"amount": 12.5,
				},
				Locale: domain.LocaleEn,
			},
			wantErrText: "mail address",
		},
		{
			name: "unsupported kind",
			from: domain.MailMessageFrom{
				FromName:    "app",
				FromAddress: "no-reply@example.test",
			},
			message: domain.MailMessage{
				Kind:   domain.MessageKind(99),
				To:     "user@example.test",
				Locale: domain.LocaleEn,
			},
			wantErrText: "unsupported message kind",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sender := newTestSender(t, tc.from)
			_, err := sender.prepareMessage(tc.message)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErrText) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErrText, err)
			}
			er, ok := appErrors.AsErrorResponse(err)
			if !ok {
				t.Fatalf("expected ErrorResponse, got %T", err)
			}
			switch tc.name {
			case "unsupported kind":
				if er.Code != svcErrors.UnsupportedNotificationKindError {
					t.Fatalf("error code = %d; want %d", er.Code, svcErrors.UnsupportedNotificationKindError)
				}
			default:
				if er.Code != svcErrors.InvalidNotificationRecipientError {
					t.Fatalf("error code = %d; want %d", er.Code, svcErrors.InvalidNotificationRecipientError)
				}
			}
		})
	}
}

func TestSenderGetMailTemplate_ReturnsParseErrorForMissingTemplateDir(t *testing.T) {
	bootstrap.InitLogger(t)

	translator, err := tools.NewTranslator("../../../data/mailer/i18n", "en")
	if err != nil {
		t.Fatalf("translator: %v", err)
	}
	sender := NewSender(
		nil,
		domain.MailMessageFrom{FromName: "app", FromAddress: "no-reply@example.test"},
		translator,
		time.Second,
		t.TempDir(),
		"app",
		"https://app.example",
	)

	_, err = sender.getMailTemplate("missing.html", domain.MailMessage{Locale: domain.LocaleEn})
	if err == nil {
		t.Fatal("expected error")
	}
}

func newTestSender(t testing.TB, from domain.MailMessageFrom) *Sender {
	t.Helper()

	translator, err := tools.NewTranslator("../../../data/mailer/i18n", "en")
	if err != nil {
		t.Fatalf("translator: %v", err)
	}

	return NewSender(
		nil,
		from,
		translator,
		time.Second,
		"../../../data/mailer/html",
		"app",
		"https://app.example",
	)
}
