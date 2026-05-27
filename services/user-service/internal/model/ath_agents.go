package model

// ATHAgent stores agent registrations for the ATH protocol
type ATHAgent struct {
	ID                uint64 `gorm:"column:id;primary_key" json:"id"`
	ClientID          string `gorm:"column:client_id;type:varchar(64);not null;uniqueIndex" json:"clientId"`
	ClientSecret      string `gorm:"column:client_secret;type:varchar(128);not null" json:"clientSecret"`
	AgentID           string `gorm:"column:agent_id;type:varchar(256);not null;uniqueIndex" json:"agentId"`
	PublicKey         string `gorm:"column:public_key;type:text;not null" json:"publicKey"`
	Name              string `gorm:"column:name;type:varchar(256);not null" json:"name"`
	DeveloperInfo     string `gorm:"column:developer_info;type:text" json:"developerInfo"`
	RedirectURIs      string `gorm:"column:redirect_uris;type:text" json:"redirectUris"`
	AllowedScopes     string `gorm:"column:allowed_scopes;type:text" json:"allowedScopes"`
	ApprovedProviders string `gorm:"column:approved_providers;type:text" json:"approvedProviders"`
	Status            string `gorm:"column:status;type:varchar(16);not null;default:'approved'" json:"status"`
	ApprovalExpiresAt int    `gorm:"column:approval_expires_at;type:bigint" json:"approvalExpiresAt"`
	CreatedAt         int    `gorm:"column:created_at;type:bigint;not null" json:"createdAt"`
	UpdatedAt         int    `gorm:"column:updated_at;type:bigint;not null" json:"updatedAt"`
}

// ATHAgentColumnNames whitelist for custom query fields
var ATHAgentColumnNames = map[string]bool{
	"id":                 true,
	"client_id":          true,
	"client_secret":      true,
	"agent_id":           true,
	"public_key":         true,
	"name":               true,
	"developer_info":     true,
	"redirect_uris":      true,
	"allowed_scopes":     true,
	"approved_providers": true,
	"status":             true,
	"approval_expires_at": true,
	"created_at":         true,
	"updated_at":         true,
}
