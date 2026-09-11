package controller

import (
	"errors"
	"net/http"

	"go-web/logger"
	"go-web/model"
	"go-web/resp"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type listUsersQuery struct {
	Page int `form:"page" binding:"omitempty,min=1"`
	Size int `form:"size" binding:"omitempty,min=1,max=1000"`
}

type createUserRequest struct {
	Name string `json:"name" binding:"required,min=1,max=64"`
}

func GetUsers(c *gin.Context) {
	var q listUsersQuery
	if err := resp.BindQuery(c, &q); err != nil {
		return
	}
	if q.Page == 0 {
		q.Page = 1
	}
	if q.Size == 0 {
		q.Size = 10
	}

	users, total, err := model.ListUsers(q.Page, q.Size)
	if err != nil {
		logger.Log.Error("list users failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgListUsersFailed)
		return
	}
	resp.OKWithPagination(c, users, resp.Pagination{
		Page:  q.Page,
		Size:  q.Size,
		Total: int(total),
	})
}

func CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}

	user := &model.User{Name: req.Name}
	if err := user.Insert(); err != nil {
		logger.Log.Error("create user failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgCreateUserFailed)
		return
	}
	resp.Created(c, user)
}

func updateUserStatus(c *gin.Context, status int, action string) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}

	user, err := model.GetUserByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			resp.Fail(c, http.StatusNotFound, resp.CodeNotFound, resp.MsgListUsersFailed)
			return
		}
		logger.Log.Error(action+" user failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgListUsersFailed)
		return
	}

	if user.Status == status {
		resp.OK(c, gin.H{"status": status})
		return
	}

	if err := user.UpdateStatus(status); err != nil {
		logger.Log.Error(action+" user failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgListUsersFailed)
		return
	}
	resp.OK(c, gin.H{"status": status})
}

func BanUser(c *gin.Context) {
	updateUserStatus(c, model.StatusDisabled, "ban")
}

func FreezeUser(c *gin.Context) {
	updateUserStatus(c, model.StatusFrozen, "freeze")
}

func DeleteUser(c *gin.Context) {
	updateUserStatus(c, model.StatusDeleted, "delete")
}

func RestoreUser(c *gin.Context) {
	updateUserStatus(c, model.StatusEnabled, "restore")
}


