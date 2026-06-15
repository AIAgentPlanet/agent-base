package dao

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"agent-base/services/user-service/internal/model"
)

const athAuditAdvisoryLockID int64 = 0x4154484155444954

type ATHAuditRecordDao interface {
	Append(ctx context.Context, builder func(sequence uint64, previousHash string) (*model.ATHAuditRecord, error)) (*model.ATHAuditRecord, error)
	ListByHandshake(ctx context.Context, handshakeID string, limit int) ([]*model.ATHAuditRecord, error)
	ListAll(ctx context.Context, limit int) ([]*model.ATHAuditRecord, error)
	Latest(ctx context.Context) (*model.ATHAuditRecord, error)
}

type athAuditRecordDao struct {
	db *gorm.DB
}

func NewATHAuditRecordDao(db *gorm.DB) ATHAuditRecordDao {
	return &athAuditRecordDao{db: db}
}

func (d *athAuditRecordDao) Append(ctx context.Context, builder func(sequence uint64, previousHash string) (*model.ATHAuditRecord, error)) (*model.ATHAuditRecord, error) {
	var created *model.ATHAuditRecord
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", athAuditAdvisoryLockID).Error; err != nil {
			return err
		}
		var latest model.ATHAuditRecord
		err := tx.Order("sequence DESC").First(&latest).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		sequence := uint64(1)
		previousHash := ""
		if err == nil {
			sequence = latest.Sequence + 1
			previousHash = latest.RecordHash
		}
		record, err := builder(sequence, previousHash)
		if err != nil {
			return err
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		outboxPayload, err := json.Marshal(map[string]interface{}{
			"event_id": record.EventID, "sequence": record.Sequence,
			"record_hash": record.RecordHash, "previous_hash": record.PreviousHash,
			"signature": record.Signature, "signing_key_id": record.SigningKeyID,
			"timestamp": record.CreatedAt,
		})
		if err != nil {
			return err
		}
		now := time.Now().Unix()
		outbox := &model.ATHAuditOutbox{
			AuditRecordID: record.ID, EventID: record.EventID,
			Sequence: record.Sequence, RecordHash: record.RecordHash,
			Payload: string(outboxPayload), Status: "pending",
			NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(outbox).Error; err != nil {
			return err
		}
		created = record
		return nil
	})
	return created, err
}

func (d *athAuditRecordDao) ListByHandshake(ctx context.Context, handshakeID string, limit int) ([]*model.ATHAuditRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var records []*model.ATHAuditRecord
	err := d.db.WithContext(ctx).
		Where("handshake_id = ?", handshakeID).
		Order("sequence ASC").Limit(limit).Find(&records).Error
	return records, err
}

func (d *athAuditRecordDao) ListAll(ctx context.Context, limit int) ([]*model.ATHAuditRecord, error) {
	query := d.db.WithContext(ctx).Order("sequence ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var records []*model.ATHAuditRecord
	return records, query.Find(&records).Error
}

func (d *athAuditRecordDao) Latest(ctx context.Context) (*model.ATHAuditRecord, error) {
	var record model.ATHAuditRecord
	if err := d.db.WithContext(ctx).Order("sequence DESC").First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}
