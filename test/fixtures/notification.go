package fixtures

import (
	"testing"

	"notification-service/internal/domain"

	commonNotificationQueue "github.com/whagency/core-common/services/notification/queue"
)

// OTPMessage returns a valid shared OTP queue fixture.
func OTPMessage() commonNotificationQueue.OTPMessage {
	return commonNotificationQueue.OTPMessage{
		Purpose:      "registrationOtp",
		ContactType:  "email",
		ContactValue: "user@example.test",
		Code:         "123456",
		Locale:       "en",
	}
}

// PushMessage returns a valid shared push queue fixture.
func PushMessage(purpose string) commonNotificationQueue.PushMessage {
	return commonNotificationQueue.PushMessage{
		Purpose: purpose,
		Data: map[string]any{
			"userId": "42",
			"email":  "user@example.test",
			"asset":  "USDT",
			"amount": 12.5,
		},
		Locale: "en",
	}
}

// SendCommand returns a valid notification command fixture.
func SendCommand(kind domain.MessageKind, addresses ...domain.Address) domain.SendCommand {
	return domain.SendCommand{
		Kind:   kind,
		To:     addresses,
		Locale: domain.LocaleEn,
		Data: domain.TemplateData{
			"userId":     "42",
			"email":      "user@example.test",
			"asset":      "USDT",
			"amount":     12.5,
			"CodeDigits": []string{"1", "2", "3", "4", "5", "6"},
		},
	}
}

// RequireAddress asserts and returns the address at index.
func RequireAddress(t testing.TB, addresses []domain.Address, index int) domain.Address {
	t.Helper()

	if index >= len(addresses) {
		t.Fatalf("address index %d out of range, len=%d", index, len(addresses))
	}
	return addresses[index]
}
