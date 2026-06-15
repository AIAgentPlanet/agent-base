package dao

import (
	"context"
	"time"

	"gorm.io/gorm"

	"agent-base/services/user-service/internal/model"
)

type ATHAuditOutboxDao interface {
	ClaimBatch(ctx context.Context, limit int, lockTTL time.Duration) ([]*model.ATHAuditOutbox, error)
	MarkDelivered(ctx context.Context, id uint64) error
	MarkFailed(ctx context.Context, id uint64, message string, retryAt time.Time) error
	RetryFailed(ctx context.Context) (int64, error)
	Status(ctx context.Context) (map[string]int64, error)
}

type athAuditOutboxDao struct {
	db *gorm.DB
}

func NewATHAuditOutboxDao(db *gorm.DB) ATHAuditOutboxDao {
	return &athAuditOutboxDao{db: db}
}

func (d *athAuditOutboxDao) ClaimBatch(ctx context.Context, limit int, lockTTL time.Duration) ([]*model.ATHAuditOutbox, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	now := time.Now().Unix()
	lockedUntil := time.Now().Add(lockTTL).Unix()
	var records []*model.ATHAuditOutbox
	err := d.db.WithContext(ctx).Raw(`
WITH candidates AS (
    SELECT id FROM ath_audit_outboxes
    WHERE status IN ('pending', 'failed', 'processing')
      AND next_attempt_at <= ?
      AND locked_until <= ?
    ORDER BY sequence ASC
    FOR UPDATE SKIP LOCKED
    LIMIT ?
)
UPDATE ath_audit_outboxes AS outbox
SET status = 'processing', locked_until = ?, updated_at = ?
FROM candidates
WHERE outbox.id = candidates.id
RETURNING outbox.*
`, now, now, limit, lockedUntil, now).Scan(&records).Error
	return records, err
}

func (d *athAuditOutboxDao) MarkDelivered(ctx context.Context, id uint64) error {
	now := time.Now().Unix()
	return d.db.WithContext(ctx).Model(&model.ATHAuditOutbox{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": "delivered", "delivered_at": now, "locked_until": 0,
		"last_error": "", "updated_at": now,
	}).Error
}

func (d *athAuditOutboxDao) MarkFailed(ctx context.Context, id uint64, message string, retryAt time.Time) error {
	now := time.Now().Unix()
	return d.db.WithContext(ctx).Model(&model.ATHAuditOutbox{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": "failed", "attempts": gorm.Expr("attempts + 1"),
		"next_attempt_at": retryAt.Unix(), "locked_until": 0,
		"last_error": message, "updated_at": now,
	}).Error
}

func (d *athAuditOutboxDao) RetryFailed(ctx context.Context) (int64, error) {
	now := time.Now().Unix()
	result := d.db.WithContext(ctx).Model(&model.ATHAuditOutbox{}).
		Where("status IN ('failed', 'processing')").
		Updates(map[string]interface{}{
			"status": "pending", "next_attempt_at": now,
			"locked_until": 0, "updated_at": now,
		})
	return result.RowsAffected, result.Error
}

func (d *athAuditOutboxDao) Status(ctx context.Context) (map[string]int64, error) {
	type count struct {
		Status string
		Total  int64
	}
	var counts []count
	if err := d.db.WithContext(ctx).Model(&model.ATHAuditOutbox{}).
		Select("status, count(*) AS total").Group("status").Scan(&counts).Error; err != nil {
		return nil, err
	}
	result := map[string]int64{"pending": 0, "processing": 0, "failed": 0, "delivered": 0}
	for _, item := range counts {
		result[item.Status] = item.Total
	}
	return result, nil
}
