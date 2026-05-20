package model

// OAuthClient oauth client model
type OAuthClient struct {
	ID            uint64 `gorm:"column:id;type:int(11);primary_key" json:"id"`
	ClientID      string `gorm:"column:client_id;type:varchar(128);not null;uniqueIndex" json:"clientId"`
	ClientSecret  string `gorm:"column:client_secret;type:varchar(256);not null" json:"clientSecret"`
	Name          string `gorm:"column:name;type:varchar(256);not null" json:"name"`
	RedirectURIs  string `gorm:"column:redirect_uris;type:text;not null" json:"redirectUris"`
	AllowedGrants string `gorm:"column:allowed_grants;type:text;not null" json:"allowedGrants"`
	AllowedScopes string `gorm:"column:allowed_scopes;type:text;not null" json:"allowedScopes"`
	UserID        uint64 `gorm:"column:user_id;type:int(11);not null;index" json:"userId"`
	Status        int    `gorm:"column:status;type:int(11);not null" json:"status"`
	CreatedAt     int    `gorm:"column:created_at;type:int(11);not null" json:"createdAt"`
	UpdatedAt     int    `gorm:"column:updated_at;type:int(11);not null" json:"updatedAt"`
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
