package handlers

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"notification-service/internal/domain"
	svcErrors "notification-service/internal/domain/errors"

	jsoniter "github.com/json-iterator/go"
	"github.com/whagency/core-common/logger"
	"github.com/whagency/core-common/queue"
	commonNotificationQueue "github.com/whagency/core-common/services/notification/queue"
)

// _otpAllowedChannels lists queue channels supported by OTP messages.
var _otpAllowedChannels = []domain.Channel{
	domain.ChannelEmail,
	domain.ChannelSMS,
}

type otpMessage commonNotificationQueue.OTPMessage

// decode converts the OTPMessage to the domain.SendCommand type.
func decodeOTPMessage(message commonNotificationQueue.OTPMessage) (domain.SendCommand, error) {
	return otpMessage(message).decode()
}

// decode converts the OTPMessage to the domain.SendCommand type.
func (m otpMessage) decode() (domain.SendCommand, error) {
	kind, err := m.decodeMessageKind()
	if err != nil {
		return domain.SendCommand{}, fmt.Errorf("[decode]%w", err)
	}

	channel, err := m.decodeChannel()
	if err != nil {
		return domain.SendCommand{}, fmt.Errorf("[decode]%w", err)
	}

	contactValue, err := m.decodeContactValue()
	if err != nil {
		return domain.SendCommand{}, fmt.Errorf("[decode]%w", err)
	}

	code, err := m.decodeCode()
	if err != nil {
		return domain.SendCommand{}, fmt.Errorf("[decode]%w", err)
	}

	locale, err := m.decodeLocale()
	if err != nil {
		return domain.SendCommand{}, fmt.Errorf("[decode]%w", err)
	}

	return domain.SendCommand{
		Kind: kind,
		To: []domain.Address{
			{
				Channel: channel,
				Value:   contactValue,
			},
		},
		Data: domain.TemplateData{
			"CodeDigits": splitCodeDigits(code),
		},
		Locale: locale,
	}, nil
}

// OTPHandler handles OTP messages.
type OTPHandler struct {
	ctrl domain.NotificationController
}

// NewOTPHandler creates a new instance of OTPHandler.
func NewOTPHandler(ctrl domain.NotificationController) *OTPHandler {
	return &OTPHandler{
		ctrl: ctrl,
	}
}

// Handle handles the OTP messages.
func (h OTPHandler) Handle(ctx context.Context, msgType queue.MessageType, data []byte) error {
	logger.LogCtx(ctx).Debugf("[OTPHandler:Handle][type: %s][msg: %s]", msgType.String(), string(data))

	if msgType != "" && msgType != commonNotificationQueue.MessageTypeOTP {
		return queue.MarkNonRetryable(fmt.Errorf("[OTPHandler:Handle][err: unexpected message type %q: %w]", msgType.String(), svcErrors.ErrMessageBadFormat))
	}

	var otp commonNotificationQueue.OTPMessage
	if err := jsoniter.Unmarshal(data, &otp); err != nil {
		return queue.MarkNonRetryable(fmt.Errorf("[OTPHandler:Handle][err: %w]", svcErrors.ErrMessageBadFormat))
	}

	cmd, err := decodeOTPMessage(otp)
	if err != nil {
		return queue.MarkNonRetryable(fmt.Errorf("[OTPHandler:Handle]%w", err))
	}

	if err = h.ctrl.Send(ctx, cmd); err != nil {
		return classifyNotificationQueueError(fmt.Errorf("[OTPHandler:Handle]%w", err))
	}

	return nil
}

// decodeChannel maps the contact type string to domain.Channel.
func (m otpMessage) decodeChannel() (domain.Channel, error) {
	channel, errChannel := domain.NewChannelFromContactField(m.ContactType)
	if errChannel != nil {
		return 0, fmt.Errorf("[decodeChannel]%w", errChannel)
	}
	if !slices.Contains(_otpAllowedChannels, channel) {
		return 0, fmt.Errorf("[decodeChannel][err: unsupported otp channel %q]", m.ContactType)
	}

	return channel, nil
}

// decodeMessageKind maps the action string to domain.MessageKind.
func (m otpMessage) decodeMessageKind() (domain.MessageKind, error) {
	messageKind, errMessageKind := domain.NewOTPMessageKindFromPurpose(m.Purpose)
	if errMessageKind != nil {
		return 0, fmt.Errorf("[decodeMessageKind]%w", errMessageKind)
	}

	return messageKind, nil
}

// decodeLocale maps the locale string to domain.Locale.
func (m otpMessage) decodeLocale() (domain.Locale, error) {
	if strings.TrimSpace(m.Locale) == "" {
		return "", nil
	}

	return domain.NewLocaleFromString(m.Locale), nil
}

// decodeContactValue validates recipient contact value for OTP messages.
func (m otpMessage) decodeContactValue() (string, error) {
	contactValue := strings.TrimSpace(m.ContactValue)
	if contactValue == "" {
		return "", fmt.Errorf("[decodeContactValue][err: empty contact value]")
	}

	return contactValue, nil
}

// decodeCode validates OTP code before template rendering.
func (m otpMessage) decodeCode() (string, error) {
	code := strings.TrimSpace(m.Code)
	if code == "" {
		return "", fmt.Errorf("[decodeCode][err: empty code]")
	}

	return code, nil
}

// splitCodeDigits splits an OTP code into individual digits for templates.
func splitCodeDigits(code string) []string {
	digits := make([]string, 0, len(code))
	for _, r := range code {
		digits = append(digits, string(r))
	}

	return digits
}
