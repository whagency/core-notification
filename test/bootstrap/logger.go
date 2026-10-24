package bootstrap

import (
	"testing"

	"github.com/whagency/core-common/testkit/loggertest"
)

// InitLogger initializes a minimal shared logger for repository tests.
func InitLogger(t testing.TB) {
	t.Helper()

	loggertest.Init(t, "notification-service-test")
}
