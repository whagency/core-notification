package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

// CronSettings holds the scheduled-job configuration.
type CronSettings struct {
	// DedupeKeyTTL is how long a notification dedupe fence is kept.
	//
	// It must comfortably exceed the longest producer retry window, otherwise a
	// late retry finds the fence gone and delivers a duplicate. account-service
	// retries an outbox message for roughly four hours before dead-lettering it.
	DedupeKeyTTL time.Duration `envconfig:"CRON_DEDUPE_KEY_TTL" default:"48h"`

	// DedupeKeyCleanupSpec is how often expired fences are deleted.
	DedupeKeyCleanupSpec string `envconfig:"CRON_DEDUPE_KEY_CLEANUP_SPEC" default:"@every 1h"`
}

var cronSettings CronSettings

// Cron returns the cron settings.
func Cron() CronSettings {
	return cronSettings
}

// loadCronConfig loads the cron settings from the environment.
func loadCronConfig() error {
	return envconfig.Process("", &cronSettings)
}
