package handlers

import (
	svcErrors "notification-service/internal/domain/errors"

	appErrors "github.com/whagency/core-common/errorsx"
	"github.com/whagency/core-common/queue"
)

// classifyNotificationQueueError maps controller errors to queue retry
// semantics. Permanent errors are parked in DLQ immediately; transient or mixed
// errors stay retryable and are handled by the shared RabbitMQ retry topology.
func classifyNotificationQueueError(err error) error {
	if err == nil {
		return nil
	}
	if isPermanentNotificationErrorTree(err) {
		return queue.MarkNonRetryable(err)
	}
	return err
}

// there isPermanentNotificationErrorTree checks if all leaf errors in the error tree are permanent and non-retryable.
func isPermanentNotificationErrorTree(err error) bool {
	leafErrors := collectLeafErrors(err)
	if len(leafErrors) == 0 {
		return false
	}

	for _, leafErr := range leafErrors {
		if !isPermanentNotificationLeafError(leafErr) {
			return false
		}
	}

	return true
}

// isPermanentNotificationLeafError checks if a single error is permanent and non-retryable.
func isPermanentNotificationLeafError(err error) bool {
	er, ok := appErrors.AsErrorResponse(err)
	if !ok {
		return false
	}

	switch er.Code {
	case appErrors.InvalidRequestError, appErrors.RecordNotFoundError:
		return true
	case svcErrors.InvalidNotificationPayloadError,
		svcErrors.UnsupportedNotificationChannelError,
		svcErrors.UnsupportedNotificationKindError,
		svcErrors.InvalidNotificationRecipientError,
		svcErrors.NotificationProfileMismatchError:
		return true
	default:
		return false
	}
}

// collectLeafErrors collects all leaf errors from a tree of errors.
func collectLeafErrors(err error) []error {
	if err == nil {
		return nil
	}

	type unwrapMany interface {
		Unwrap() []error
	}
	type unwrapOne interface {
		Unwrap() error
	}

	if multi, ok := err.(unwrapMany); ok {
		unwrapped := multi.Unwrap()
		if len(unwrapped) == 0 {
			return []error{err}
		}

		result := make([]error, 0, len(unwrapped))
		for _, item := range unwrapped {
			result = append(result, collectLeafErrors(item)...)
		}
		return result
	}

	if single, ok := err.(unwrapOne); ok {
		unwrapped := single.Unwrap()
		if unwrapped == nil {
			return []error{err}
		}
		return collectLeafErrors(unwrapped)
	}

	return []error{err}
}
