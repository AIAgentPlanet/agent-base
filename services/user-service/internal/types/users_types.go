package types

import (
	"time"

	"github.com/go-dev-frame/sponge/pkg/sgorm/query"
)

var _ time.Time

// Tip: suggested filling in the binding rules https://github.com/go-playground/validator in request struct fields tag.


// CreateUsersRequest request params
type CreateUsersRequest struct {
	Username  string `json:"username" binding:""`
	Password  string `json:"password" binding:""`
	Email  string `json:"email" binding:""`
	Phone  string `json:"phone" binding:""`
	Nickname  string `json:"nickname" binding:""`
	Avatar  string `json:"avatar" binding:""`
	Status  int `json:"status" binding:""`
	LoginAt  int `json:"loginAt" binding:""`
}

// UpdateUsersByIDRequest request params
type UpdateUsersByIDRequest struct {
	ID uint64 `json:"id" binding:""` // uint64 id

	Username  string `json:"username" binding:""`
	Password  string `json:"password" binding:""`
	Email  string `json:"email" binding:""`
	Phone  string `json:"phone" binding:""`
	Nickname  string `json:"nickname" binding:""`
	Avatar  string `json:"avatar" binding:""`
	Status  int `json:"status" binding:""`
	LoginAt  int `json:"loginAt" binding:""`
}

// UsersObjDetail detail
type UsersObjDetail struct {
	ID uint64 `json:"id"` // convert to uint64 id

	CreatedAt  int `json:"createdAt"`
	UpdatedAt  int `json:"updatedAt"`
	Username  string `json:"username"`
	Password  string `json:"-"`
	Email  string `json:"email"`
	Phone  string `json:"phone"`
	Nickname  string `json:"nickname"`
	Avatar  string `json:"avatar"`
	Status  int `json:"status"`
	LoginAt  int `json:"loginAt"`
}


// CreateUsersReply only for api docs
type CreateUsersReply struct {
	Code int    `json:"code"` // return code
	Msg  string `json:"msg"`  // return information description
	Data struct {
		ID uint64 `json:"id"` // id
	} `json:"data"` // return data
}

// UpdateUsersByIDReply only for api docs
type UpdateUsersByIDReply struct {
	Code int      `json:"code"` // return code
	Msg  string   `json:"msg"`  // return information description
	Data struct{} `json:"data"` // return data
}

// GetUsersByIDReply only for api docs
type GetUsersByIDReply struct {
	Code int    `json:"code"` // return code
	Msg  string `json:"msg"`  // return information description
	Data struct {
		Users UsersObjDetail `json:"users"`
	} `json:"data"` // return data
}

// DeleteUsersByIDReply only for api docs
type DeleteUsersByIDReply struct {
	Code int      `json:"code"` // return code
	Msg  string   `json:"msg"`  // return information description
	Data struct{} `json:"data"` // return data
}

// DeleteUserssByIDsReply only for api docs
type DeleteUserssByIDsReply struct {
	Code int      `json:"code"` // return code
	Msg  string   `json:"msg"`  // return information description
	Data struct{} `json:"data"` // return data
}

// ListUserssRequest request params
type ListUserssRequest struct {
	query.Params
}

// ListUserssReply only for api docs
type ListUserssReply struct {
	Code int    `json:"code"` // return code
	Msg  string `json:"msg"`  // return information description
	Data struct {
		Userss []UsersObjDetail `json:"userss"`
	} `json:"data"` // return data
}

// DeleteUserssByIDsRequest request params
type DeleteUserssByIDsRequest struct {
	IDs []uint64 `json:"ids" binding:"min=1"` // id list
}

// GetUsersByConditionRequest request params
type GetUsersByConditionRequest struct {
	query.Conditions
}

// GetUsersByConditionReply only for api docs
type GetUsersByConditionReply struct {
	Code int    `json:"code"` // return code
	Msg  string `json:"msg"`  // return information description
	Data struct {
		Users UsersObjDetail `json:"users"`
	} `json:"data"` // return data
}

// ListUserssByIDsRequest request params
type ListUserssByIDsRequest struct {
	IDs []uint64 `json:"ids" binding:"min=1"` // id list
}

// ListUserssByIDsReply only for api docs
type ListUserssByIDsReply struct {
	Code int    `json:"code"` // return code
	Msg  string `json:"msg"`  // return information description
	Data struct {
		Userss []UsersObjDetail `json:"userss"`
	} `json:"data"` // return data
}

// ==================== Auth Related Types ====================

// RegisterRequest user registration request
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6,max=64"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone" binding:"required"`
}

// RegisterReply user registration response
type RegisterReply struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ID uint64 `json:"id"`
	} `json:"data"`
}

// LoginRequest user login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginReply user login response
type LoginReply struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Token string `json:"token"`
	} `json:"data"`
}

// ProfileReply user profile response
type ProfileReply struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Users UsersObjDetail `json:"users"`
	} `json:"data"`
}

// UpdateProfileRequest update profile request
type UpdateProfileRequest struct {
	Nickname string `json:"nickname" binding:"omitempty,max=64"`
	Email    string `json:"email" binding:"omitempty,email"`
	Phone    string `json:"phone" binding:"omitempty"`
	Avatar   string `json:"avatar" binding:"omitempty,max=512"`
}

// SendResetCodeRequest send reset code request
type SendResetCodeRequest struct {
	Email string `json:"email" binding:"required_without=Phone"`
	Phone string `json:"phone" binding:"required_without=Email"`
}

// SendResetCodeReply send reset code response
type SendResetCodeReply struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct{} `json:"data"`
}

// ResetPasswordRequest reset password request
type ResetPasswordRequest struct {
	Email       string `json:"email" binding:"required_without=Phone"`
	Phone       string `json:"phone" binding:"required_without=Email"`
	Code        string `json:"code" binding:"required,len=6"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=64"`
}

// ResetPasswordReply reset password response
type ResetPasswordReply struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct{} `json:"data"`
}
