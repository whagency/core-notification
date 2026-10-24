package errors

const _serviceCode = 500000

const (
	CustomError = _serviceCode + iota + 1
	InvalidNotificationPayloadError
	UnsupportedNotificationChannelError
	UnsupportedNotificationKindError
	InvalidNotificationRecipientError
	NotificationProfileMismatchError
)
