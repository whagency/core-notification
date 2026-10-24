package domain

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/whagency/core-common/logger"
)

type Channel uint8

const (
	ChannelEmail Channel = iota + 1
	ChannelSMS
	ChannelWebSocket
)

var ChannelContactFields = map[Channel]string{
	ChannelEmail:     "email",
	ChannelSMS:       "phone",
	ChannelWebSocket: "userId",
}

var _channelByContactField = map[string]Channel{
	"email":  ChannelEmail,
	"phone":  ChannelSMS,
	"userId": ChannelWebSocket,
}

func (c Channel) ContactField() string {
	if contactField, ok := ChannelContactFields[c]; ok {
		return contactField
	}
	return ""
}

// String returns a stable identifier for the channel, used in dedupe fence keys
// and logs. It is deliberately independent of the numeric enum value so that
// reordering the constants cannot silently repoint existing fences.
func (c Channel) String() string {
	switch c {
	case ChannelEmail:
		return "email"
	case ChannelSMS:
		return "sms"
	case ChannelWebSocket:
		return "websocket"
	default:
		return "unknown"
	}
}

// NewChannelFromContactField converts a transport contact field to Channel.
func NewChannelFromContactField(value string) (Channel, error) {
	channel, ok := _channelByContactField[strings.TrimSpace(value)]
	if ok {
		return channel, nil
	}

	return 0, fmt.Errorf("[domain][NewChannelFromContactField][err: unsupported channel contact field %q]", value)
}

type MessageKind uint8

const (
	MsgOTPRegistration MessageKind = iota + 1
	MsgOTPPasswordReset
	MsgOTPTwoFactor

	MsgPushSuccessfulDeposit
	MsgPushRequiredVerificationDeposit
	MsgPushSuccessfulWithdrawal
	MsgPushSuccessfulIncomingTransfer
	MsgPushSuccessfulOutgoingTransfer
)

var _otpMessageKindByPurpose = map[string]MessageKind{
	"registrationOtp":  MsgOTPRegistration,
	"passwordResetOtp": MsgOTPPasswordReset,
	"twoFactorOtp":     MsgOTPTwoFactor,
}

var _pushMessageKindByPurpose = map[string]MessageKind{
	"requiredVerificationDeposit": MsgPushRequiredVerificationDeposit,
	"successfulDeposit":           MsgPushSuccessfulDeposit,
	"successfulIncomingTransfer":  MsgPushSuccessfulIncomingTransfer,
	"successfulOutgoingTransfer":  MsgPushSuccessfulOutgoingTransfer,
	"successfulWithdrawal":        MsgPushSuccessfulWithdrawal,
}

var MessagePushChannels = map[MessageKind][]Channel{
	MsgPushSuccessfulDeposit: {
		ChannelWebSocket,
		ChannelEmail,
	},
	MsgPushSuccessfulWithdrawal: {
		ChannelWebSocket,
		ChannelEmail,
	},
	MsgPushSuccessfulIncomingTransfer: {
		ChannelWebSocket,
		ChannelEmail,
	},
	MsgPushSuccessfulOutgoingTransfer: {
		ChannelEmail,
	},
	MsgPushRequiredVerificationDeposit: {
		ChannelWebSocket,
	},
}

func GetPushChannelsByMessageKind(kind MessageKind) []Channel {
	if channels, ok := MessagePushChannels[kind]; ok {
		return channels
	}
	return []Channel{}
}

// NewOTPMessageKindFromPurpose converts an OTP queue purpose to MessageKind.
func NewOTPMessageKindFromPurpose(value string) (MessageKind, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return MsgOTPRegistration, nil
	}

	messageKind, ok := _otpMessageKindByPurpose[normalized]
	if ok {
		return messageKind, nil
	}

	return 0, fmt.Errorf("[domain][NewOTPMessageKindFromPurpose][err: unsupported otp purpose %q]", value)
}

// NewPushMessageKindFromPurpose converts a push queue purpose to MessageKind.
func NewPushMessageKindFromPurpose(value string) (MessageKind, error) {
	messageKind, ok := _pushMessageKindByPurpose[strings.TrimSpace(value)]
	if ok {
		return messageKind, nil
	}

	return 0, fmt.Errorf("[domain][NewPushMessageKindFromPurpose][err: unsupported push purpose %q]", value)
}

const (
	LocaleEn Locale = "en"
	LocaleUk Locale = "uk"
)

type Locale string

// String returns the string representation of the Locale.
func (l Locale) String() string {
	return string(l)
}

var _localeByString = map[string]Locale{
	string(LocaleEn): LocaleEn,
	string(LocaleUk): LocaleUk,
}

// NewLocaleFromString converts a transport string to Locale.
func NewLocaleFromString(value string) Locale {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	parts := strings.Split(value, "_")

	locale, ok := _localeByString[parts[0]]
	if ok {
		return locale
	}

	logger.LogCtx(context.Background()).Warnf("[domain][NewLocaleFromString][err: unsupported locale %s]", value)

	return LocaleEn
}

// IsSupportedLocale reports whether locale is supported.
func IsSupportedLocale(locale Locale) bool {
	_, ok := _localeByString[string(locale)]
	return ok
}

// SupportedLocaleStrings returns transport strings for supported locales.
func SupportedLocaleStrings() []string {
	return slices.Sorted(maps.Keys(_localeByString))
}

type Address struct {
	Channel Channel
	Value   string // email, phone, device token...
}

type TemplateData map[string]any

type SendCommand struct {
	Kind   MessageKind
	To     []Address
	Locale Locale
	Data   TemplateData // e.g. {"code":"123456","app":"app"}

	// DedupeKey optionally fences duplicate deliveries of the same logical
	// notification. Empty means no fencing — correct for producers that publish
	// exactly once, such as direct request-scoped sends.
	DedupeKey string
}

// GetString returns the string representation of the param by its key.
func (d TemplateData) GetString(param string) string {
	if val, ok := d[param]; ok {
		return fmt.Sprint(val)
	}
	return ""
}

// GetFloat returns the float representation of the param by its key as string.
func (d TemplateData) GetFloat(param string) float64 {
	if val, ok := d[param]; ok {
		if f, okCast := val.(float64); okCast {
			return f
		}
	}
	return 0
}

// NotificationController defines the interface for sending notifications.
type NotificationController interface {
	Send(ctx context.Context, cmd SendCommand) error
}
