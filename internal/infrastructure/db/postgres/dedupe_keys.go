package postgres

import (
	"context"
	"fmt"
	"time"

	"notification-service/tools"

	"github.com/whagency/core-common/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type dedupeKeySQL struct {
	Key       string    `gorm:"column:dedupe_key;primaryKey"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
}

func (dedupeKeySQL) TableName() string {
	return "notification_dedupe_keys"
}

// DedupeKeyRepo is the PostgreSQL implementation of domain.DedupeKeyRepository.
type DedupeKeyRepo struct {
	db *gorm.DB
}

// NewDedupeKeyRepository creates a new instance of DedupeKeyRepo.
func NewDedupeKeyRepository(db *gorm.DB) *DedupeKeyRepo {
	return &DedupeKeyRepo{db: db}
}

// Reserve claims key with a TTL-based expiry using ON CONFLICT DO NOTHING.
// Returns alreadyReserved=true when a record with that key already exists.
//
// The insert is the fence: two consumers racing on the same key mean exactly one
// INSERT affects a row, so exactly one of them proceeds to deliver.
func (r DedupeKeyRepo) Reserve(ctx context.Context, key string, ttl time.Duration) (alreadyReserved bool, err error) {
	logger.LogCtx(ctx).Debugf("[DedupeKeyRepo:Reserve][key: %s]", key)
	defer tools.EndDBRequestTimer(time.Now())

	row := dedupeKeySQL{
		Key:       key,
		ExpiresAt: time.Now().UTC().Add(ttl),
	}

	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&row)

	if result.Error != nil {
		tools.TrackMetricDBErrorAdd()
		return false, fmt.Errorf("[DedupeKeyRepo:Reserve][err:%w]", result.Error)
	}

	return result.RowsAffected == 0, nil
}

// Release removes a reservation so a failed delivery can be retried.
func (r DedupeKeyRepo) Release(ctx context.Context, key string) error {
	logger.LogCtx(ctx).Debugf("[DedupeKeyRepo:Release][key: %s]", key)
	defer tools.EndDBRequestTimer(time.Now())

	result := r.db.WithContext(ctx).
		Where("dedupe_key = ?", key).
		Delete(&dedupeKeySQL{})

	if result.Error != nil {
		tools.TrackMetricDBErrorAdd()
		return fmt.Errorf("[DedupeKeyRepo:Release][err:%w]", result.Error)
	}

	return nil
}

// DeleteExpired removes all rows whose expires_at is in the past.
func (r DedupeKeyRepo) DeleteExpired(ctx context.Context) (deleted int64, err error) {
	logger.LogCtx(ctx).Debug("[DedupeKeyRepo:DeleteExpired]")
	defer tools.EndDBRequestTimer(time.Now())

	result := r.db.WithContext(ctx).
		Where("expires_at < NOW()").
		Delete(&dedupeKeySQL{})

	if result.Error != nil {
		tools.TrackMetricDBErrorAdd()
		return 0, fmt.Errorf("[DedupeKeyRepo:DeleteExpired][err:%w]", result.Error)
	}

	return result.RowsAffected, nil
}
