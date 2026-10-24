package domain

import (
	"strings"
	"testing"
)

func TestMailMessageTemplatePathAndSubject(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		template    MailMessageTemplate
		wantPath    string
		wantSubject string
	}{
		{name: "signup", template: MailMessageTemplateSignupConfirm, wantPath: "signup_confirm_code.html", wantSubject: "subject.signup_confirm_code"},
		{name: "two factor", template: MailMessageTemplateTwoFactorConfirm, wantPath: "two_factor_confirm_code.html", wantSubject: "subject.two_factor_confirm_code"},
		{name: "deposit", template: MailMessageTemplateSuccessfulDeposit, wantPath: "successful_deposit.html", wantSubject: "subject.successful_deposit"},
		{name: "withdrawal", template: MailMessageTemplateSuccessfulWithdrawal, wantPath: "successful_withdrawal.html", wantSubject: "subject.successful_withdrawal"},
		{name: "outgoing transfer", template: MailMessageTemplateSuccessfulOutgoingTransfer, wantPath: "successful_outgoing_transfer.html", wantSubject: "subject.successful_outgoing_transfer"},
		{name: "incoming transfer", template: MailMessageTemplateSuccessfulIncomingTransfer, wantPath: "successful_incoming_transfer.html", wantSubject: "subject.successful_incoming_transfer"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.template.Path(); got != tc.wantPath {
				t.Fatalf("Path = %q; want %q", got, tc.wantPath)
			}
			if got := tc.template.Subject(); got != tc.wantSubject {
				t.Fatalf("Subject = %q; want %q", got, tc.wantSubject)
			}
		})
	}

	t.Run("unknown", func(t *testing.T) {
		t.Parallel()

		template := MailMessageTemplate(99)
		if got := template.Path(); got != "base.html" {
			t.Fatalf("Path = %q", got)
		}
		if got := template.Subject(); got != "Info" {
			t.Fatalf("Subject = %q", got)
		}
	})
}

func TestFormateSubject(t *testing.T) {
	t.Parallel()

	got := FormateSubject(":APP_NAME code at :TIME", "app")
	if !strings.Contains(got, "[app] code at ") {
		t.Fatalf("unexpected subject: %q", got)
	}
	if strings.Contains(got, ":APP_NAME") || strings.Contains(got, ":TIME") {
		t.Fatalf("expected placeholders to be replaced: %q", got)
	}
}
