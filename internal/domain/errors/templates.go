package errors

import (
	"net/http"

	"github.com/whagency/core-common/errorsx"
)

// Templates returns a map of error codes to their corresponding error templates for the account domain.
func Templates() map[int]errorsx.Template {
	return map[int]errorsx.Template{
		CustomError: {
			Code:       CustomError,
			Message:    CustomErrMessage,
			HTTPStatus: http.StatusBadRequest,
		},
		InvalidNotificationPayloadError: {
			Code:       InvalidNotificationPayloadError,
			Message:    InvalidNotificationPayloadErrMessage,
			HTTPStatus: http.StatusBadRequest,
		},
		UnsupportedNotificationChannelError: {
			Code:       UnsupportedNotificationChannelError,
			Message:    UnsupportedNotificationChannelMessage,
			HTTPStatus: http.StatusBadRequest,
		},
		UnsupportedNotificationKindError: {
			Code:       UnsupportedNotificationKindError,
			Message:    UnsupportedNotificationKindMessage,
			HTTPStatus: http.StatusBadRequest,
		},
		InvalidNotificationRecipientError: {
			Code:       InvalidNotificationRecipientError,
			Message:    InvalidNotificationRecipientErrMessage,
			HTTPStatus: http.StatusBadRequest,
		},
		NotificationProfileMismatchError: {
			Code:       NotificationProfileMismatchError,
			Message:    NotificationProfileMismatchErrMessage,
			HTTPStatus: http.StatusBadRequest,
		},
	}
}
