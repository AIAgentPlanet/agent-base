package model

type Users struct {
	ID        uint64 `gorm:"column:id;primary_key" json:"id"`
	CreatedAt int    `gorm:"column:created_at;type:bigint;not null" json:"createdAt"`
	UpdatedAt int    `gorm:"column:updated_at;type:bigint;not null" json:"updatedAt"`
	DeletedAt int    `gorm:"column:deleted_at;type:bigint" json:"deletedAt"`
	Username  string `gorm:"column:username;type:varchar(64);not null;unique" json:"username"`
	Password  string `gorm:"column:password;type:text;not null" json:"password"`
	Email     string `gorm:"column:email;type:varchar(255);not null;unique" json:"email"`
	Phone     string `gorm:"column:phone;type:varchar(32);not null;unique" json:"phone"`
	Nickname  string `gorm:"column:nickname;type:varchar(128);not null;default:''" json:"nickname"`
	Avatar    string `gorm:"column:avatar;type:text;not null;default:''" json:"avatar"`
	Status    int    `gorm:"column:status;type:integer;not null;default:1" json:"status"`
	LoginAt   int    `gorm:"column:login_at;type:bigint;not null" json:"loginAt"`
}

// UsersColumnNames Whitelist for custom query fields to prevent sql injection attacks
var UsersColumnNames = map[string]bool{
	"id":         true,
	"created_at": true,
	"updated_at": true,
	"deleted_at": true,
	"username":   true,
	"password":   true,
	"email":      true,
	"phone":      true,
	"nickname":   true,
	"avatar":     true,
	"status":     true,
	"login_at":   true,
}
