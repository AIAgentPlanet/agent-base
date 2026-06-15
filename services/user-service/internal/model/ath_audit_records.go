package model

// ATHAuditRecord is an append-only, signed link in the ATH audit hash chain.
type ATHAuditRecord struct {
	ID               uint64 `gorm:"column:id;primary_key" json:"id"`
	Sequence         uint64 `gorm:"column:sequence;type:bigint;not null;uniqueIndex" json:"sequence"`
	EventID          string `gorm:"column:event_id;type:varchar(64);not null;uniqueIndex" json:"event_id"`
	EventType        string `gorm:"column:event_type;type:varchar(64);not null;index" json:"event_type"`
	ClientID         string `gorm:"column:client_id;type:varchar(128);index" json:"client_id,omitempty"`
	AgentID          string `gorm:"column:agent_id;type:varchar(256);index" json:"agent_id,omitempty"`
	HandshakeID      string `gorm:"column:handshake_id;type:varchar(128);index" json:"handshake_id,omitempty"`
	TokenID          string `gorm:"column:token_id;type:varchar(128);index" json:"token_id,omitempty"`
	Payload          string `gorm:"column:payload;type:text;not null" json:"payload"`
	PayloadHash      string `gorm:"column:payload_hash;type:varchar(64);not null" json:"payload_hash"`
	PreviousHash     string `gorm:"column:previous_hash;type:varchar(64);not null" json:"previous_hash"`
	RecordHash       string `gorm:"column:record_hash;type:varchar(64);not null;uniqueIndex" json:"record_hash"`
	Signature        string `gorm:"column:signature;type:text;not null" json:"signature"`
	SigningKeyID     string `gorm:"column:signing_key_id;type:varchar(256);not null" json:"signing_key_id"`
	SigningPublicKey string `gorm:"column:signing_public_key;type:text;not null" json:"signing_public_key"`
	CreatedAt        int64  `gorm:"column:created_at;type:bigint;not null;index" json:"created_at"`
}

func (ATHAuditRecord) TableName() string {
	return "ath_audit_records"
}
