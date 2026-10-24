package mailer

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"path/filepath"
	"time"

	"notification-service/internal/domain"
	svcErrors "notification-service/internal/domain/errors"
	"notification-service/tools"

	appErrors "github.com/whagency/core-common/errorsx"
	"github.com/whagency/core-common/logger"
	"github.com/wneessen/go-mail"
)

type Sender struct {
	client       *mail.Client
	from         domain.MailMessageFrom
	translator   *tools.Translator
	sendTimeout  time.Duration
	templatesDir string
	appName      string
	appUrl       string
}

// NewSender creates a new instance of Sender.
func NewSender(
	client *mail.Client,
	from domain.MailMessageFrom,
	translator *tools.Translator,
	sendTimeout time.Duration,
	templatesDir string,
	appName string,
	appUrl string,
) *Sender {
	return &Sender{
		client:       client,
		from:         from,
		translator:   translator,
		sendTimeout:  sendTimeout,
		templatesDir: templatesDir,
		appName:      appName,
		appUrl:       appUrl,
	}
}

// Send it sends the mail message.
func (s Sender) Send(ctx context.Context, mailMessage domain.MailMessage) error {
	logger.LogCtx(ctx).Debugf("[mailer][Sender:Send][mailMessage: %+v]", mailMessage)

	message, errPrepare := s.prepareMessage(mailMessage)
	if errPrepare != nil {
		errPrepare.SetCause(fmt.Errorf("[mailer][Sender:Send]%w", errPrepare.Cause))
		return errPrepare
	}

	if s.client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, s.sendTimeout)
	defer cancel()

	if err := s.client.DialAndSendWithContext(ctx, message); err != nil {
		return fmt.Errorf("[mailer][Sender:Send][err: %s]", err)
	}

	return nil
}

// prepareMessage prepares the message to be sent.
func (s Sender) prepareMessage(mailMessage domain.MailMessage) (*mail.Msg, *appErrors.ErrorResponse) {
	message := mail.NewMsg()
	if err := message.FromFormat(s.from.FromName, s.from.FromAddress); err != nil {
		return nil, svcErrors.New(
			svcErrors.InvalidNotificationRecipientError,
			appErrors.WithCause(fmt.Errorf("[prepareMessage][from][err: %w]", err)),
		)
	}

	if err := message.To(mailMessage.To); err != nil {
		return nil, svcErrors.New(
			svcErrors.InvalidNotificationRecipientError,
			appErrors.WithCause(fmt.Errorf("[prepareMessage][to][err: %w]", err)),
		)
	}

	msgTemplate, err := decodeMessageTemplate(mailMessage.Kind)
	if err != nil {
		err.SetCause(fmt.Errorf("[prepareMessage]%w", err.Cause))
		return nil, err
	}

	htmlTemplate, errTemplate := s.getMailTemplate(msgTemplate.Path(), mailMessage)
	if errTemplate != nil {
		return nil, svcErrors.New(
			appErrors.InternalServerError,
			appErrors.WithCause(fmt.Errorf("[prepareMessage]%w", errTemplate)),
		)
	}

	msgSubject := domain.FormateSubject(
		s.translator.T(mailMessage.Locale.String(), msgTemplate.Subject(), nil),
		s.appName,
	)

	message.Subject(msgSubject)
	message.SetBodyString(mail.TypeTextHTML, htmlTemplate)

	if s.client == nil {
		logger.Log().Infof("[Sender:prepareMessage][mailer:LOG][from: %s][to: %s][subject: %s][message: %s]",
			message.GetFromString(),
			message.GetToString(),
			msgSubject,
			htmlTemplate,
		)
	}

	return message, nil
}

// decodeMessageTemplate returns the message template.
func decodeMessageTemplate(msgKind domain.MessageKind) (domain.MailMessageTemplate, *appErrors.ErrorResponse) {
	switch msgKind {
	case domain.MsgOTPRegistration:
		return domain.MailMessageTemplateSignupConfirm, nil
	case domain.MsgOTPTwoFactor, domain.MsgOTPPasswordReset:
		return domain.MailMessageTemplateTwoFactorConfirm, nil
	case domain.MsgPushSuccessfulDeposit:
		return domain.MailMessageTemplateSuccessfulDeposit, nil
	case domain.MsgPushSuccessfulWithdrawal:
		return domain.MailMessageTemplateSuccessfulWithdrawal, nil
	case domain.MsgPushSuccessfulOutgoingTransfer:
		return domain.MailMessageTemplateSuccessfulOutgoingTransfer, nil
	case domain.MsgPushSuccessfulIncomingTransfer:
		return domain.MailMessageTemplateSuccessfulIncomingTransfer, nil
	default:
		return 0, svcErrors.New(
			svcErrors.UnsupportedNotificationKindError,
			appErrors.WithCause(fmt.Errorf("[decodeMessageTemplate][kind: %d][err: unsupported message kind]", msgKind)),
		)
	}
}

// getMailTemplate returns the mail template.
func (s Sender) getMailTemplate(messageTemplate string, mailMessage domain.MailMessage) (string, error) {
	layoutPath := filepath.Join(s.templatesDir, "layout.html")
	bodyPath := filepath.Join(s.templatesDir, "templates", messageTemplate)

	tmpl, err := template.
		New("layout").
		Funcs(template.FuncMap{
			"trimZeros": tools.TrimZeros,
			"t": func(key string, data interface{}) string {
				return s.translator.T(mailMessage.Locale.String(), key, data)
			},
		}).
		ParseFiles(layoutPath, bodyPath)
	if err != nil {
		return "", fmt.Errorf("[getMailTemplate][err: %w]", err)
	}

	var buf bytes.Buffer
	if err = tmpl.ExecuteTemplate(&buf, "layout", domain.TemplateData{
		"Message": mailMessage,
		"AppName": s.appName,
		"AppUrl":  s.appUrl,
		"Time":    time.Now().UTC().Format(time.DateTime) + " (UTC)",
		"Year":    time.Now().Year(),
	}); err != nil {
		return "", fmt.Errorf("[getMailTemplate][err: %w]", err)
	}

	return buf.String(), nil
}
