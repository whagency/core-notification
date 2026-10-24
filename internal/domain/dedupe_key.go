package domain

import (
	"context"
	"time"
)

// DedupeKeyRepository fences duplicate notification deliveries.
//
// Producers that publish from a transactional outbox deliver at-least-once: a
// publish that succeeded before its status update failed is republished on the
// next attempt. Without a fence the user simply receives the notification twice.
type DedupeKeyRepository interface {
	// Reserve claims key for the given TTL. Returns alreadyReserved=true when the
	// key is already present, meaning this notification was already handled.
	Reserve(ctx context.Context, key string, ttl time.Duration) (alreadyReserved bool, err error)
	// Release removes a reservation so a failed delivery can be retried.
	Release(ctx context.Context, key string) error
	// DeleteExpired removes all rows whose expires_at is in the past.
	DeleteExpired(ctx context.Context) (deleted int64, err error)
}
