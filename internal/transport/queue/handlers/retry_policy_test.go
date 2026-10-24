package handlers

import (
	"errors"
	"fmt"
	"testing"

	svcErrors "notification-service/internal/domain/errors"

	appErrors "github.com/whagency/core-common/errorsx"
	"github.com/whagency/core-common/queue"
)

func TestClassifyNotificationQueueError(t *testing.T) {
	t.Run("marks single permanent error as non-retryable", func(t *testing.T) {
		err := fmt.Errorf("[PushHandler:Handle]%w", svcErrors.New(appErrors.RecordNotFoundError))

		got := classifyNotificationQueueError(err)
		if !errors.Is(got, queue.NonRetryableErr) {
			t.Fatalf("expected non-retryable error, got %v", got)
		}
	})

	t.Run("marks joined permanent errors as non-retryable", func(t *testing.T) {
		err := fmt.Errorf(
			"[NotificationCtrl:Send]%w",
			errors.Join(
				svcErrors.New(appErrors.RecordNotFoundError),
				svcErrors.New(appErrors.InvalidRequestError),
			),
		)

		got := classifyNotificationQueueError(err)
		if !errors.Is(got, queue.NonRetryableErr) {
			t.Fatalf("expected non-retryable error, got %v", got)
		}
	})

	t.Run("keeps mixed permanent and transient errors retryable", func(t *testing.T) {
		err := fmt.Errorf(
			"[NotificationCtrl:Send]%w",
			errors.Join(
				svcErrors.New(appErrors.RecordNotFoundError),
				errors.New("smtp unavailable"),
			),
		)

		got := classifyNotificationQueueError(err)
		if errors.Is(got, queue.NonRetryableErr) {
			t.Fatalf("expected retryable error, got %v", got)
		}
	})

	t.Run("marks notification service permanent codes as non-retryable", func(t *testing.T) {
		permanentCodes := []int{
			svcErrors.InvalidNotificationPayloadError,
			svcErrors.UnsupportedNotificationChannelError,
			svcErrors.UnsupportedNotificationKindError,
			svcErrors.InvalidNotificationRecipientError,
			svcErrors.NotificationProfileMismatchError,
		}

		for _, code := range permanentCodes {
			err := fmt.Errorf("[PushHandler:Handle]%w", svcErrors.New(code))

			got := classifyNotificationQueueError(err)
			if !errors.Is(got, queue.NonRetryableErr) {
				t.Fatalf("code %d: expected non-retryable error, got %v", code, got)
			}
		}
	})

	t.Run("keeps external service errors retryable", func(t *testing.T) {
		err := fmt.Errorf("[NotificationCtrl:Send]%w", svcErrors.New(appErrors.ExternalServiceError))

		got := classifyNotificationQueueError(err)
		if errors.Is(got, queue.NonRetryableErr) {
			t.Fatalf("expected retryable error, got %v", got)
		}
	})

	t.Run("keeps nil nil", func(t *testing.T) {
		if got := classifyNotificationQueueError(nil); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})
}
