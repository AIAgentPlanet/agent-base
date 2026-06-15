package model

// ATHAuditOutbox reliably delivers signed audit heads to an external anchor.
type ATHAuditOutbox struct {
	ID            uint64 `gorm:"column:id;primary_key" json:"id"`
	AuditRecordID uint64 `gorm:"column:audit_record_id;type:bigint;not null;uniqueIndex" json:"audit_record_id"`
	EventID       string `gorm:"column:event_id;type:varchar(64);not null;uniqueIndex" json:"event_id"`
	Sequence      uint64 `gorm:"column:sequence;type:bigint;not null;index" json:"sequence"`
	RecordHash    string `gorm:"column:record_hash;type:varchar(64);not null" json:"record_hash"`
	Payload       string `gorm:"column:payload;type:text;not null" json:"payload"`
	Status        string `gorm:"column:status;type:varchar(16);not null;index" json:"status"`
	Attempts      int    `gorm:"column:attempts;type:integer;not null;default:0" json:"attempts"`
	NextAttemptAt int64  `gorm:"column:next_attempt_at;type:bigint;not null;index" json:"next_attempt_at"`
	LockedUntil   int64  `gorm:"column:locked_until;type:bigint;not null;default:0" json:"locked_until"`
	LastError     string `gorm:"column:last_error;type:text" json:"last_error,omitempty"`
	DeliveredAt   int64  `gorm:"column:delivered_at;type:bigint;not null;default:0" json:"delivered_at,omitempty"`
	CreatedAt     int64  `gorm:"column:created_at;type:bigint;not null" json:"created_at"`
	UpdatedAt     int64  `gorm:"column:updated_at;type:bigint;not null" json:"updated_at"`
}

func (ATHAuditOutbox) TableName() string {
	return "ath_audit_outboxes"
}
