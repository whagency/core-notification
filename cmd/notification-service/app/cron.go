package app

import (
	"context"
	"fmt"

	"notification-service/config"

	"github.com/whagency/core-common/cron"
	"github.com/whagency/core-common/logger"
)

// setupCronManager registers the application's scheduled jobs.
func (app *Application) setupCronManager() error {
	app.cronManager = cron.NewManager()

	// Notification dedupe fences are only useful for as long as a producer might
	// still retry. Past that they are dead weight on every Reserve, so they are
	// swept on a schedule rather than accumulating forever.
	err := app.cronManager.AddFunc(
		config.Cron().DedupeKeyCleanupSpec,
		"notification.dedupe_key.cleanup",
		func(ctx context.Context) error {
			deleted, cleanupErr := app.Core().DedupeKeys.DeleteExpired(ctx)
			if cleanupErr != nil {
				return cleanupErr
			}
			if deleted > 0 {
				logger.LogCtx(ctx).Infof("[notification][Cron:dedupe_key.cleanup][rows: %d]", deleted)
			}
			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("[setupCronManager]%w", err)
	}

	return nil
}
