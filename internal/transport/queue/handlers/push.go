package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"notification-service/internal/domain"
	svcErrors "notification-service/internal/domain/errors"

	jsoniter "github.com/json-iterator/go"
	"github.com/whagency/core-common/logger"
	"github.com/whagency/core-common/queue"
	commonNotificationQueue "github.com/whagency/core-common/services/notification/queue"
)

type pushMessage commonNotificationQueue.PushMessage

func decodePushMessage(message commonNotificationQueue.PushMessage) (domain.SendCommand, error) {
	return pushMessage(message).decode()
}

// decode converts the PushMessage to the domain.SendCommand type.
func (m pushMessage) decode() (domain.SendCommand, error) {
	kind, err := m.decodeMessageKind()
	if err != nil {
		return domain.SendCommand{}, fmt.Errorf("[decode]%w", err)
	}

	data, err := m.decodeData()
	if err != nil {
		return domain.SendCommand{}, fmt.Errorf("[decode]%w", err)
	}

	addresses, err := m.decodeAddresses(kind, data)
	if err != nil {
		return domain.SendCommand{}, fmt.Errorf("[decode]%w", err)
	}

	locale, err := m.decodeLocale()
	if err != nil {
		return domain.SendCommand{}, fmt.Errorf("[decode]%w", err)
	}

	return domain.SendCommand{
		Kind:      kind,
		To:        addresses,
		Data:      data,
		Locale:    locale,
		DedupeKey: strings.TrimSpace(m.DedupeKey),
	}, nil
}

// PushHandler handles push notification messages.
type PushHandler struct {
	ctrl domain.NotificationController
}

// NewPushHandler creates a new instance of PushHandler.
func NewPushHandler(ctrl domain.NotificationController) *PushHandler {
	return &PushHandler{
		ctrl: ctrl,
	}
}

// Handle handles the push notification messages.
func (h PushHandler) Handle(ctx context.Context, msgType queue.MessageType, data []byte) error {
	logger.LogCtx(ctx).Debugf("[PushHandler:Handle][type: %s][msg: %s]", msgType.String(), string(data))

	if msgType != "" && msgType != commonNotificationQueue.MessageTypePush {
		return queue.MarkNonRetryable(fmt.Errorf("[PushHandler:Handle][err: unexpected message type %q: %w]", msgType.String(), svcErrors.ErrMessageBadFormat))
	}

	var push commonNotificationQueue.PushMessage
	if err := jsoniter.Unmarshal(data, &push); err != nil {
		return queue.MarkNonRetryable(fmt.Errorf("[PushHandler:Handle][err: %w]", svcErrors.ErrMessageBadFormat))
	}

	cmd, err := decodePushMessage(push)
	if err != nil {
		return queue.MarkNonRetryable(fmt.Errorf("[PushHandler:Handle]%w", err))
	}

	if err = h.ctrl.Send(ctx, cmd); err != nil {
		return classifyNotificationQueueError(fmt.Errorf("[PushHandler:Handle]%w", err))
	}

	return nil
}

// decodeData validates template payload presence for push notifications.
func (m pushMessage) decodeData() (domain.TemplateData, error) {
	if m.Data == nil {
		return nil, fmt.Errorf("[decodeData][err: empty data]")
	}

	return domain.TemplateData(m.Data), nil
}

// decodeAddresses maps the message kind to a slice of domain.Address.
func (m pushMessage) decodeAddresses(kind domain.MessageKind, data domain.TemplateData) ([]domain.Address, error) {
	channels := domain.GetPushChannelsByMessageKind(kind)
	addresses := make([]domain.Address, 0, len(channels))

	for _, channel := range channels {
		contactField := decodeTemplateString(data, channel.ContactField())
		switch channel {
		case domain.ChannelWebSocket:
			if _, err := decodeTemplateUint64(data, channel.ContactField()); err != nil {
				return nil, fmt.Errorf("[decodeAddresses]%w", err)
			}
		case domain.ChannelEmail:
			if contactField == "" {
				if _, err := decodeTemplateUint64(data, domain.ChannelWebSocket.ContactField()); err != nil {
					return nil, fmt.Errorf("[decodeAddresses][email_fallback]%w", err)
				}
			}
		}

		address := domain.Address{
			Channel: channel,
			Value:   contactField,
		}
		addresses = append(addresses, address)
	}

	return addresses, nil
}

// decodeMessageKind maps the action string to domain.MessageKind.
func (m pushMessage) decodeMessageKind() (domain.MessageKind, error) {
	messageKind, errMessageKind := domain.NewPushMessageKindFromPurpose(m.Purpose)
	if errMessageKind != nil {
		return 0, fmt.Errorf("[decodeMessageKind]%w", errMessageKind)
	}

	return messageKind, nil
}

// decodeLocale maps the locale string to domain.Locale.
func (m pushMessage) decodeLocale() (domain.Locale, error) {
	if strings.TrimSpace(m.Locale) == "" {
		return "", nil
	}

	return domain.NewLocaleFromString(m.Locale), nil
}

// decodeTemplateString returns a trimmed string representation for a template field.
func decodeTemplateString(data domain.TemplateData, key string) string {
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}

	return strings.TrimSpace(fmt.Sprint(value))
}

// decodeTemplateUint64 returns a uint64 representation for a template field.
func decodeTemplateUint64(data domain.TemplateData, key string) (uint64, error) {
	value := decodeTemplateString(data, key)
	if value == "" {
		return 0, fmt.Errorf("[decodeTemplateUint64][err: empty %s value]", key)
	}

	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("[decodeTemplateUint64][err: invalid %s value]", key)
	}

	return parsed, nil
}
