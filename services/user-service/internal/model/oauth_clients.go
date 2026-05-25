package model

// OAuthClient oauth client model
type OAuthClient struct {
	ID            uint64 `gorm:"column:id;primary_key" json:"id"`
	ClientID      string `gorm:"column:client_id;type:varchar(128);not null;unique" json:"clientId"`
	ClientSecret  string `gorm:"column:client_secret;type:varchar(256);not null" json:"clientSecret"`
	Name          string `gorm:"column:name;type:varchar(256);not null" json:"name"`
	RedirectURIs  string `gorm:"column:redirect_uris;type:text;not null" json:"redirectUris"`
	AllowedGrants string `gorm:"column:allowed_grants;type:text;not null;default:'[\"authorization_code\"]'" json:"allowedGrants"`
	AllowedScopes string `gorm:"column:allowed_scopes;type:text;not null;default:'[\"profile\"]'" json:"allowedScopes"`
	UserID        uint64 `gorm:"column:user_id;type:bigint;not null;index" json:"userId"`
	Status        int    `gorm:"column:status;type:integer;not null;default:1" json:"status"`
	CreatedAt     int    `gorm:"column:created_at;type:bigint;not null" json:"createdAt"`
	UpdatedAt     int    `gorm:"column:updated_at;type:bigint;not null" json:"updatedAt"`
}

// OAuthClientColumnNames whitelist for custom query fields
var OAuthClientColumnNames = map[string]bool{
	"id":             true,
	"client_id":      true,
	"client_secret":  true,
	"name":           true,
	"redirect_uris":  true,
	"allowed_grants": true,
	"allowed_scopes": true,
	"user_id":        true,
	"status":         true,
	"created_at":     true,
	"updated_at":     true,
}
