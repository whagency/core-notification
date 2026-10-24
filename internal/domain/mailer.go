package domain

import (
	"context"
	"strings"
	"time"
)

type MailMessageTemplate uint16

const (
	MailMessageTemplateSignupConfirm MailMessageTemplate = iota + 1
	MailMessageTemplateTwoFactorConfirm
	MailMessageTemplateSuccessfulDeposit
	MailMessageTemplateSuccessfulWithdrawal
	MailMessageTemplateSuccessfulOutgoingTransfer
	MailMessageTemplateSuccessfulIncomingTransfer
)

var mailMessageTemplateToPath = map[MailMessageTemplate]string{
	MailMessageTemplateSignupConfirm:              "signup_confirm_code.html",
	MailMessageTemplateTwoFactorConfirm:           "two_factor_confirm_code.html",
	MailMessageTemplateSuccessfulDeposit:          "successful_deposit.html",
	MailMessageTemplateSuccessfulWithdrawal:       "successful_withdrawal.html",
	MailMessageTemplateSuccessfulOutgoingTransfer: "successful_outgoing_transfer.html",
	MailMessageTemplateSuccessfulIncomingTransfer: "successful_incoming_transfer.html",
}

var mailMessageTemplateToSubject = map[MailMessageTemplate]string{
	MailMessageTemplateSignupConfirm:              "subject.signup_confirm_code",
	MailMessageTemplateTwoFactorConfirm:           "subject.two_factor_confirm_code",
	MailMessageTemplateSuccessfulDeposit:          "subject.successful_deposit",
	MailMessageTemplateSuccessfulWithdrawal:       "subject.successful_withdrawal",
	MailMessageTemplateSuccessfulOutgoingTransfer: "subject.successful_outgoing_transfer",
	MailMessageTemplateSuccessfulIncomingTransfer: "subject.successful_incoming_transfer",
}

// Path returns the path to the template file.
func (m MailMessageTemplate) Path() string {
	if val, ok := mailMessageTemplateToPath[m]; ok {
		return val
	}
	return "base.html"
}

// Subject returns the subject of the email.
func (m MailMessageTemplate) Subject() string {
	if val, ok := mailMessageTemplateToSubject[m]; ok {
		return val
	}
	return "Info"
}

// FormateSubject returns the subject of the email with replaced params.
func FormateSubject(subject string, appName string) string {
	replacer := strings.NewReplacer(
		":APP_NAME", "["+appName+"]",
		":TIME", time.Now().UTC().Format(time.DateTime)+" (UTC)",
	)

	return replacer.Replace(subject)
}

type MailMessageFrom struct {
	FromName    string
	FromAddress string
}

type MailMessage struct {
	To     string
	Params TemplateData
	Kind   MessageKind
	Locale Locale
}

// MailerManager defines the interface for sending emails.
type MailerManager interface {
	Send(context.Context, MailMessage) error
}
