package model

type Users struct {
	ID        uint64 `gorm:"column:id;type:int(11);primary_key" json:"id"`
	CreatedAt int    `gorm:"column:created_at;type:int(11);not null" json:"createdAt"`
	UpdatedAt int    `gorm:"column:updated_at;type:int(11);not null" json:"updatedAt"`
	DeletedAt int    `gorm:"column:deleted_at;type:int(11)" json:"deletedAt"`
	Username  string `gorm:"column:username;type:text;not null" json:"username"`
	Password  string `gorm:"column:password;type:text;not null" json:"password"`
	Email     string `gorm:"column:email;type:text;not null" json:"email"`
	Phone     string `gorm:"column:phone;type:text;not null" json:"phone"`
	Nickname  string `gorm:"column:nickname;type:text;not null" json:"nickname"`
	Avatar    string `gorm:"column:avatar;type:text;not null" json:"avatar"`
	Status    int    `gorm:"column:status;type:int(11);not null" json:"status"`
	LoginAt   int    `gorm:"column:login_at;type:int(11);not null" json:"loginAt"`
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


