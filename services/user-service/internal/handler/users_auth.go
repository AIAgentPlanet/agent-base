package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/go-dev-frame/sponge/pkg/gin/middleware"
	"github.com/go-dev-frame/sponge/pkg/gin/response"
	"github.com/go-dev-frame/sponge/pkg/logger"
	"github.com/go-dev-frame/sponge/pkg/sgorm/query"

	"agent-base/services/user-service/internal/ecode"
	"agent-base/services/user-service/internal/model"
	"agent-base/services/user-service/internal/pkg/code"
	"agent-base/services/user-service/internal/pkg/hash"
	"agent-base/services/user-service/internal/pkg/jwt"
	"agent-base/services/user-service/internal/types"
)

// Register user registration
// @Summary User registration
// @Description Register a new user with username, password, email and phone
// @Tags users
// @Accept json
// @Produce json
// @Param data body types.RegisterRequest true "registration information"
// @Success 200 {object} types.RegisterReply{}
// @Router /api/v1/users/register [post]
func (h *usersHandler) Register(c *gin.Context) {
	form := &types.RegisterRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		logger.Warn("ShouldBindJSON error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}

	ctx := middleware.WrapCtx(c)

	// check if username exists
	_, err := h.iDao.GetByCondition(ctx, &query.Conditions{
		Columns: []query.Column{
			{Name: "username", Value: form.Username},
		},
	})
	if err == nil {
		response.Error(c, ecode.ErrUserExists)
		return
	}

	// check if email exists
	_, err = h.iDao.GetByCondition(ctx, &query.Conditions{
		Columns: []query.Column{
			{Name: "email", Value: form.Email},
		},
	})
	if err == nil {
		response.Error(c, ecode.ErrUserExists)
		return
	}

	// check if phone exists
	_, err = h.iDao.GetByCondition(ctx, &query.Conditions{
		Columns: []query.Column{
			{Name: "phone", Value: form.Phone},
		},
	})
	if err == nil {
		response.Error(c, ecode.ErrUserExists)
		return
	}

	// hash password
	pwdHash, err := hash.HashPassword(form.Password)
	if err != nil {
		logger.Error("HashPassword error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	// create user
	user := &model.Users{
		Username:  form.Username,
		Password:  pwdHash,
		Email:     form.Email,
		Phone:     form.Phone,
		Nickname:  form.Username,
		Avatar:    "",
		Status:    2, // activated
		LoginAt:   0,
		CreatedAt: int(time.Now().Unix()),
		UpdatedAt: int(time.Now().Unix()),
	}

	if err := h.iDao.Create(ctx, user); err != nil {
		logger.Error("Create user error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	response.Success(c, gin.H{"id": user.ID})
}

// Login user login
// @Summary User login
// @Description Login with username and password, return JWT token
// @Tags users
// @Accept json
// @Produce json
// @Param data body types.LoginRequest true "login credentials"
// @Success 200 {object} types.LoginReply{}
// @Router /api/v1/users/login [post]
func (h *usersHandler) Login(c *gin.Context) {
	form := &types.LoginRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		logger.Warn("ShouldBindJSON error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}

	ctx := middleware.WrapCtx(c)

	// find user by username
	user, err := h.iDao.GetByCondition(ctx, &query.Conditions{
		Columns: []query.Column{
			{Name: "username", Value: form.Username},
		},
	})
	if err != nil {
		logger.Warn("GetByCondition error", logger.Err(err), logger.String("username", form.Username), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.ErrUserNotFound)
		return
	}

	// check password
	if !hash.CheckPassword(form.Password, user.Password) {
		response.Error(c, ecode.ErrInvalidPassword)
		return
	}

	// check status
	if user.Status != 2 {
		response.Error(c, ecode.ErrUnauthorized)
		return
	}

	// generate JWT token
	token, err := jwt.GenerateToken(user.ID)
	if err != nil {
		logger.Error("GenerateToken error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	// update login time
	user.LoginAt = int(time.Now().Unix())
	_ = h.iDao.UpdateByID(ctx, &model.Users{
		ID:      user.ID,
		LoginAt: user.LoginAt,
	})

	response.Success(c, gin.H{"token": token})
}

// GetProfile get current user profile
// @Summary Get user profile
// @Description Get current logged in user profile
// @Tags users
// @Accept json
// @Produce json
// @Success 200 {object} types.ProfileReply{}
// @Router /api/v1/users/profile [get]
// @Security BearerAuth
func (h *usersHandler) GetProfile(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		response.Error(c, ecode.ErrUnauthorized)
		return
	}

	ctx := middleware.WrapCtx(c)
	user, err := h.iDao.GetByID(ctx, userID)
	if err != nil {
		logger.Warn("GetByID error", logger.Err(err), logger.Uint64("id", userID), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.ErrUserNotFound)
		return
	}

	data, err := convertUsers(user)
	if err != nil {
		response.Error(c, ecode.ErrGetByIDUsers)
		return
	}

	response.Success(c, gin.H{"users": data})
}

// UpdateProfile update current user profile
// @Summary Update user profile
// @Description Update current logged in user profile
// @Tags users
// @Accept json
// @Produce json
// @Param data body types.UpdateProfileRequest true "profile information"
// @Success 200 {object} types.UpdateUsersByIDReply{}
// @Router /api/v1/users/profile [put]
// @Security BearerAuth
func (h *usersHandler) UpdateProfile(c *gin.Context) {
	userID, err := getUserIDFromContext(c)
	if err != nil {
		response.Error(c, ecode.ErrUnauthorized)
		return
	}

	form := &types.UpdateProfileRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		logger.Warn("ShouldBindJSON error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}

	ctx := middleware.WrapCtx(c)
	updateData := &model.Users{
		ID:        userID,
		UpdatedAt: int(time.Now().Unix()),
	}
	if form.Nickname != "" {
		updateData.Nickname = form.Nickname
	}
	if form.Email != "" {
		updateData.Email = form.Email
	}
	if form.Phone != "" {
		updateData.Phone = form.Phone
	}
	if form.Avatar != "" {
		updateData.Avatar = form.Avatar
	}

	if err := h.iDao.UpdateByID(ctx, updateData); err != nil {
		logger.Error("UpdateByID error", logger.Err(err), logger.Uint64("id", userID), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	response.Success(c)
}

// SendResetCode send password reset verification code
// @Summary Send reset code
// @Description Send verification code to user's email or phone for password reset
// @Tags users
// @Accept json
// @Produce json
// @Param data body types.SendResetCodeRequest true "reset code request"
// @Success 200 {object} types.SendResetCodeReply{}
// @Router /api/v1/users/reset-code [post]
func (h *usersHandler) SendResetCode(c *gin.Context) {
	form := &types.SendResetCodeRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		logger.Warn("ShouldBindJSON error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}

	ctx := middleware.WrapCtx(c)

	// find user by email or phone
	var target string
	var user *model.Users
	var err error

	if form.Email != "" {
		target = form.Email
		user, err = h.iDao.GetByCondition(ctx, &query.Conditions{
			Columns: []query.Column{
				{Name: "email", Value: form.Email},
			},
		})
	} else if form.Phone != "" {
		target = form.Phone
		user, err = h.iDao.GetByCondition(ctx, &query.Conditions{
			Columns: []query.Column{
				{Name: "phone", Value: form.Phone},
			},
		})
	}

	if err != nil || user == nil {
		// for security, return success even if user not found
		logger.Warn("SendResetCode user not found", logger.String("target", target), middleware.GCtxRequestIDField(c))
		response.Success(c)
		return
	}

	// generate and save code
	vcode := code.GenerateCode()
	if err := code.SaveResetCode(ctx, target, vcode); err != nil {
		logger.Error("SaveResetCode error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.ErrSendCodeFailed)
		return
	}

	// simulate sending code (log output)
	logger.Info("SendResetCode",
		logger.String("target", target),
		logger.String("code", vcode),
		logger.Uint64("user_id", user.ID),
		middleware.GCtxRequestIDField(c),
	)

	response.Success(c)
}

// ResetPassword reset password with verification code
// @Summary Reset password
// @Description Reset password using verification code sent to email or phone
// @Tags users
// @Accept json
// @Produce json
// @Param data body types.ResetPasswordRequest true "reset password request"
// @Success 200 {object} types.ResetPasswordReply{}
// @Router /api/v1/users/reset-password [post]
func (h *usersHandler) ResetPassword(c *gin.Context) {
	form := &types.ResetPasswordRequest{}
	if err := c.ShouldBindJSON(form); err != nil {
		logger.Warn("ShouldBindJSON error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.InvalidParams.RewriteMsg(formatValidationError(err)))
		return
	}

	ctx := middleware.WrapCtx(c)

	// determine target
	var target string
	if form.Email != "" {
		target = form.Email
	} else if form.Phone != "" {
		target = form.Phone
	}

	// verify code
	ok, err := code.VerifyResetCode(ctx, target, form.Code)
	if err != nil {
		logger.Error("VerifyResetCode error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}
	if !ok {
		response.Error(c, ecode.ErrInvalidCode)
		return
	}

	// find user
	var user *model.Users
	if form.Email != "" {
		user, err = h.iDao.GetByCondition(ctx, &query.Conditions{
			Columns: []query.Column{
				{Name: "email", Value: form.Email},
			},
		})
	} else {
		user, err = h.iDao.GetByCondition(ctx, &query.Conditions{
			Columns: []query.Column{
				{Name: "phone", Value: form.Phone},
			},
		})
	}
	if err != nil {
		logger.Warn("ResetPassword user not found", logger.String("target", target), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.ErrUserNotFound)
		return
	}

	// hash new password
	pwdHash, err := hash.HashPassword(form.NewPassword)
	if err != nil {
		logger.Error("HashPassword error", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	// update password
	if err := h.iDao.UpdateByID(ctx, &model.Users{
		ID:        user.ID,
		Password:  pwdHash,
		UpdatedAt: int(time.Now().Unix()),
	}); err != nil {
		logger.Error("UpdateByID error", logger.Err(err), logger.Uint64("id", user.ID), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	response.Success(c)
}

// getUserIDFromContext extract user id from JWT context
func getUserIDFromContext(c *gin.Context) (uint64, error) {
	authHeader := c.GetHeader("Authorization")
	if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
		return 0, ecode.ErrUnauthorized.Err()
	}
	token := authHeader[7:]
	userID, err := jwt.ParseToken(token)
	if err != nil {
		return 0, ecode.ErrUnauthorized.Err()
	}
	return userID, nil
}
