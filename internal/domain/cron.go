package domain

import "context"

// CronManager schedules and runs the service's background jobs.
type CronManager interface {
	Start()
	AddFunc(string, string, func(context.Context) error) error
	Stop() context.Context
}
