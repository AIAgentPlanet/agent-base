package handler

import (
	"errors"
	"math"

	"github.com/gin-gonic/gin"

	"github.com/go-dev-frame/sponge/pkg/copier"
	"github.com/go-dev-frame/sponge/pkg/gin/middleware"
	"github.com/go-dev-frame/sponge/pkg/gin/response"
	"github.com/go-dev-frame/sponge/pkg/logger"
	"github.com/go-dev-frame/sponge/pkg/utils"

	"agent-base/services/user-service/internal/cache"
	"agent-base/services/user-service/internal/dao"
	"agent-base/services/user-service/internal/database"
	"agent-base/services/user-service/internal/ecode"
	"agent-base/services/user-service/internal/model"
	"agent-base/services/user-service/internal/types"
)

var _ UsersHandler = (*usersHandler)(nil)

// UsersHandler defining the handler interface
type UsersHandler interface {
	Create(c *gin.Context)
	DeleteByID(c *gin.Context)
	UpdateByID(c *gin.Context)
	GetByID(c *gin.Context)
	List(c *gin.Context)

	DeleteByIDs(c *gin.Context)
	GetByCondition(c *gin.Context)
	ListByIDs(c *gin.Context)
	ListByLastID(c *gin.Context)

	// Auth related handlers
	Register(c *gin.Context)
	Login(c *gin.Context)
	GetProfile(c *gin.Context)
	UpdateProfile(c *gin.Context)
	SendResetCode(c *gin.Context)
	ResetPassword(c *gin.Context)

	// OAuth related handlers
	CreateOAuthClient(c *gin.Context)
	ListOAuthClients(c *gin.Context)
	UpdateOAuthClient(c *gin.Context)
	DeleteOAuthClient(c *gin.Context)
	OAuthAuthorize(c *gin.Context)
	OAuthToken(c *gin.Context)
	OAuthUserInfo(c *gin.Context)
	OAuthRevoke(c *gin.Context)

	// ATH protocol handlers
	ATHDiscovery(c *gin.Context)
	ATHRegister(c *gin.Context)
	ATHAgentStatus(c *gin.Context)
	ATHAuthorize(c *gin.Context)
	ATHToken(c *gin.Context)
	ATHRevoke(c *gin.Context)
	ATHProxy(c *gin.Context)
}

type usersHandler struct {
	iDao dao.UsersDao
}

// NewUsersHandler creating the handler interface
func NewUsersHandler() UsersHandler {
	return &usersHandler{
		iDao: dao.NewUsersDao(
			database.GetDB(), // db driver is sqlite
			cache.NewUsersCache(database.GetCacheType()),
		),
	}
}

// Create a new users
// @Summary Create a new users
// @Description Creates a new users entity using the provided data in the request body.
// @Tags users
// @Accept json
// @Produce json
// @Param data body types.CreateUsersRequest true "users information"
// @Success 200 {object} types.CreateUsersReply{}
// @Router /api/v1/users [post]
// @Security BearerAuth
func (h *usersHandler) Create(c *gin.Context) {
	form := &types.CreateUsersRequest{}
	err := c.ShouldBindJSON(form)
	if err != nil {
		logger.Warn("ShouldBindJSON error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.InvalidParams)
		return
	}

	users := &model.Users{}
	err = copier.Copy(users, form)
	if err != nil {
		response.Error(c, ecode.ErrCreateUsers)
		return
	}
	// Note: if copier.Copy cannot assign a value to a field, add it here

	ctx := middleware.WrapCtx(c)
	err = h.iDao.Create(ctx, users)
	if err != nil {
		logger.Error("Create error", logger.Err(err), logger.Any("form", form), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	response.Success(c, gin.H{"id": users.ID})
}

// DeleteByID delete a users by id
// @Summary Delete a users by id
// @Description Deletes a existing users identified by the given id in the path.
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} types.DeleteUsersByIDReply{}
// @Router /api/v1/users/{id} [delete]
// @Security BearerAuth
func (h *usersHandler) DeleteByID(c *gin.Context) {
	_, id, isAbort := getUsersIDFromPath(c)
	if isAbort {
		response.Error(c, ecode.InvalidParams)
		return
	}

	ctx := middleware.WrapCtx(c)
	err := h.iDao.DeleteByID(ctx, id)
	if err != nil {
		logger.Error("DeleteByID error", logger.Err(err), logger.Any("id", id), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	response.Success(c)
}

// UpdateByID update a users by id
// @Summary Update a users by id
// @Description Updates the specified users by given id in the path, support partial update.
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Param data body types.UpdateUsersByIDRequest true "users information"
// @Success 200 {object} types.UpdateUsersByIDReply{}
// @Router /api/v1/users/{id} [put]
// @Security BearerAuth
func (h *usersHandler) UpdateByID(c *gin.Context) {
	_, id, isAbort := getUsersIDFromPath(c)
	if isAbort {
		response.Error(c, ecode.InvalidParams)
		return
	}

	form := &types.UpdateUsersByIDRequest{}
	err := c.ShouldBindJSON(form)
	if err != nil {
		logger.Warn("ShouldBindJSON error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.InvalidParams)
		return
	}
	form.ID = id

	users := &model.Users{}
	err = copier.Copy(users, form)
	if err != nil {
		response.Error(c, ecode.ErrUpdateByIDUsers)
		return
	}
	// Note: if copier.Copy cannot assign a value to a field, add it here

	ctx := middleware.WrapCtx(c)
	err = h.iDao.UpdateByID(ctx, users)
	if err != nil {
		logger.Error("UpdateByID error", logger.Err(err), logger.Any("form", form), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	response.Success(c)
}

// GetByID get a users by id
// @Summary Get a users by id
// @Description Gets detailed information of a users specified by the given id in the path.
// @Tags users
// @Param id path string true "id"
// @Accept json
// @Produce json
// @Success 200 {object} types.GetUsersByIDReply{}
// @Router /api/v1/users/{id} [get]
// @Security BearerAuth
func (h *usersHandler) GetByID(c *gin.Context) {
	_, id, isAbort := getUsersIDFromPath(c)
	if isAbort {
		response.Error(c, ecode.InvalidParams)
		return
	}

	ctx := middleware.WrapCtx(c)
	users, err := h.iDao.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, database.ErrRecordNotFound) {
			logger.Warn("GetByID not found", logger.Err(err), logger.Any("id", id), middleware.GCtxRequestIDField(c))
			response.Error(c, ecode.NotFound)
		} else {
			logger.Error("GetByID error", logger.Err(err), logger.Any("id", id), middleware.GCtxRequestIDField(c))
			response.Output(c, ecode.InternalServerError.ToHTTPCode())
		}
		return
	}

	data := &types.UsersObjDetail{}
	err = copier.Copy(data, users)
	if err != nil {
		response.Error(c, ecode.ErrGetByIDUsers)
		return
	}
	// Note: if copier.Copy cannot assign a value to a field, add it here

	response.Success(c, gin.H{"users": data})
}

// List get a paginated list of userss by custom conditions
// @Summary Get a paginated list of userss by custom conditions
// @Description Returns a paginated list of users based on query filters, including page number and size.
// @Tags users
// @Accept json
// @Produce json
// @Param data body types.Params true "query parameters"
// @Success 200 {object} types.ListUserssReply{}
// @Router /api/v1/users/list [post]
// @Security BearerAuth
func (h *usersHandler) List(c *gin.Context) {
	form := &types.ListUserssRequest{}
	err := c.ShouldBindJSON(form)
	if err != nil {
		logger.Warn("ShouldBindJSON error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.InvalidParams)
		return
	}

	ctx := middleware.WrapCtx(c)
	userss, total, err := h.iDao.GetByColumns(ctx, &form.Params)
	if err != nil {
		logger.Error("GetByColumns error", logger.Err(err), logger.Any("form", form), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	data, err := convertUserss(userss)
	if err != nil {
		response.Error(c, ecode.ErrListUsers)
		return
	}

	response.Success(c, gin.H{
		"userss": data,
		"total":        total,
	})
}

// DeleteByIDs batch delete users by ids
// @Summary Batch delete users by ids
// @Description Deletes multiple users by a list of id
// @Tags users
// @Param data body types.DeleteUserssByIDsRequest true "id array"
// @Accept json
// @Produce json
// @Success 200 {object} types.DeleteUserssByIDsReply{}
// @Router /api/v1/users/delete/ids [post]
// @Security BearerAuth
func (h *usersHandler) DeleteByIDs(c *gin.Context) {
	form := &types.DeleteUserssByIDsRequest{}
	err := c.ShouldBindJSON(form)
	if err != nil {
		logger.Warn("ShouldBindJSON error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.InvalidParams)
		return
	}

	ctx := middleware.WrapCtx(c)
	err = h.iDao.DeleteByIDs(ctx, form.IDs)
	if err != nil {
		logger.Error("GetByIDs error", logger.Err(err), logger.Any("form", form), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	response.Success(c)
}

// GetByCondition get a users by custom condition
// @Summary Get a users by custom condition
// @Description Returns a single users that matches the specified filter conditions.
// @Tags users
// @Param data body types.Conditions true "query condition"
// @Accept json
// @Produce json
// @Success 200 {object} types.GetUsersByConditionReply{}
// @Router /api/v1/users/condition [post]
// @Security BearerAuth
func (h *usersHandler) GetByCondition(c *gin.Context) {
	form := &types.GetUsersByConditionRequest{}
	err := c.ShouldBindJSON(form)
	if err != nil {
		logger.Warn("ShouldBindJSON error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.InvalidParams)
		return
	}
	err = form.Conditions.CheckValid()
	if err != nil {
		logger.Warn("Parameters error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.InvalidParams)
		return
	}

	ctx := middleware.WrapCtx(c)
	users, err := h.iDao.GetByCondition(ctx, &form.Conditions)
	if err != nil {
		if errors.Is(err, database.ErrRecordNotFound) {
			logger.Warn("GetByCondition not found", logger.Err(err), logger.Any("form", form), middleware.GCtxRequestIDField(c))
			response.Error(c, ecode.NotFound)
		} else {
			logger.Error("GetByCondition error", logger.Err(err), logger.Any("form", form), middleware.GCtxRequestIDField(c))
			response.Output(c, ecode.InternalServerError.ToHTTPCode())
		}
		return
	}

	data := &types.UsersObjDetail{}
	err = copier.Copy(data, users)
	if err != nil {
		response.Error(c, ecode.ErrGetByIDUsers)
		return
	}
	// Note: if copier.Copy cannot assign a value to a field, add it here

	response.Success(c, gin.H{"users": data})
}

// ListByIDs batch get users by ids
// @Summary Batch get users by ids
// @Description Returns a list of users that match the list of id.
// @Tags users
// @Param data body types.ListUserssByIDsRequest true "id array"
// @Accept json
// @Produce json
// @Success 200 {object} types.ListUserssByIDsReply{}
// @Router /api/v1/users/list/ids [post]
// @Security BearerAuth
func (h *usersHandler) ListByIDs(c *gin.Context) {
	form := &types.ListUserssByIDsRequest{}
	err := c.ShouldBindJSON(form)
	if err != nil {
		logger.Warn("ShouldBindJSON error: ", logger.Err(err), middleware.GCtxRequestIDField(c))
		response.Error(c, ecode.InvalidParams)
		return
	}

	ctx := middleware.WrapCtx(c)
	usersMap, err := h.iDao.GetByIDs(ctx, form.IDs)
	if err != nil {
		logger.Error("GetByIDs error", logger.Err(err), logger.Any("form", form), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	userss := []*types.UsersObjDetail{}
	for _, id := range form.IDs {
		if v, ok := usersMap[id]; ok {
			record, err := convertUsers(v)
			if err != nil {
				response.Error(c, ecode.ErrListUsers)
				return
			}
			userss = append(userss, record)
		}
	}

	response.Success(c, gin.H{
		"userss": userss,
	})
}

// ListByLastID get a paginated list of userss by last id
// @Summary Get a paginated list of userss by last id
// @Description Returns a paginated list of userss starting after a given last id, useful for cursor-based pagination.
// @Tags users
// @Accept json
// @Produce json
// @Param lastID query int false "last id, default is MaxInt32" default(0)
// @Param limit query int false "number per page" default(10)
// @Param sort query string false "sort by column name of table, and the "-" sign before column name indicates reverse order" default(-id)
// @Success 200 {object} types.ListUserssReply{}
// @Router /api/v1/users/list [get]
// @Security BearerAuth
func (h *usersHandler) ListByLastID(c *gin.Context) {
	lastID := utils.StrToUint64(c.Query("lastID"))
	if lastID == 0 {
		lastID = math.MaxInt32
	}
	limit := utils.StrToInt(c.Query("limit"))
	if limit == 0 {
		limit = 10
	}
	sort := c.Query("sort")

	ctx := middleware.WrapCtx(c)
	userss, err := h.iDao.GetByLastID(ctx, lastID, limit, sort)
	if err != nil {
		logger.Error("GetByLastID error", logger.Err(err), logger.Uint64("lastID", lastID), logger.Int("limit", limit), middleware.GCtxRequestIDField(c))
		response.Output(c, ecode.InternalServerError.ToHTTPCode())
		return
	}

	data, err := convertUserss(userss)
	if err != nil {
		response.Error(c, ecode.ErrListByLastIDUsers)
		return
	}

	response.Success(c, gin.H{
		"userss": data,
	})
}

func getUsersIDFromPath(c *gin.Context) (string, uint64, bool) {
	idStr := c.Param("id")
	id, err := utils.StrToUint64E(idStr)
	if err != nil || id == 0 {
		logger.Warn("StrToUint64E error: ", logger.String("idStr", idStr), middleware.GCtxRequestIDField(c))
		return "", 0, true
	}

	return idStr, id, false
}

func convertUsers(users *model.Users) (*types.UsersObjDetail, error) {
	data := &types.UsersObjDetail{}
	err := copier.Copy(data, users)
	if err != nil {
		return nil, err
	}
	// Note: if copier.Copy cannot assign a value to a field, add it here

	return data, nil
}

func convertUserss(fromValues []*model.Users) ([]*types.UsersObjDetail, error) {
	toValues := []*types.UsersObjDetail{}
	for _, v := range fromValues {
		data, err := convertUsers(v)
		if err != nil {
			return nil, err
		}
		toValues = append(toValues, data)
	}

	return toValues, nil
}
