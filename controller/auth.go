package controller

import (
	"errors"
	"net/http"

	"go-web/authz"
	"go-web/logger"
	"go-web/middleware"
	"go-web/model"
	"go-web/resp"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type loginRequest struct {
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type registerRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=32"`
	Password string `json:"password" binding:"required,min=6,max=64"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func Register(c *gin.Context) {
	var req registerRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}

	if _, err := model.GetUserByName(req.Name); err == nil {
		resp.Fail(c, http.StatusConflict, resp.CodeUserExists, resp.MsgUserExists)
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Log.Error("register query failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgRegisterFailed)
		return
	}

	user := &model.User{
		Name:     req.Name,
		Role:     model.RoleUser,
		VipLevel: model.VipFree,
		Status:   model.StatusEnabled,
	}
	if err := user.SetPassword(req.Password); err != nil {
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgRegisterFailed)
		return
	}
	if err := user.Insert(); err != nil {
		logger.Log.Error("register insert failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgRegisterFailed)
		return
	}

	pair, err := model.IssueTokenPair(user)
	if err != nil {
		logger.Log.Error("register issue token failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgRegisterFailed)
		return
	}

	resp.Created(c, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    pair.ExpiresIn,
		"user":          user,
		"capabilities":  authz.Caps(user.Role, user.VipLevel),
	})
}

func checkUserStatus(c *gin.Context, user *model.User) bool {
	switch user.Status {
	case model.StatusEnabled:
		return true
	case model.StatusDisabled:
		resp.Fail(c, http.StatusForbidden, resp.CodeAuthBanned, resp.MsgAuthBanned)
		return false
	case model.StatusFrozen:
		resp.Fail(c, http.StatusForbidden, resp.CodeAuthFrozen, resp.MsgAuthFrozen)
		return false
	case model.StatusDeleted:
		resp.Fail(c, http.StatusUnauthorized, resp.CodeAuthInvalid, resp.MsgLoginFailed)
		return false
	default:
		resp.Fail(c, http.StatusForbidden, resp.CodeAuthDisabled, resp.MsgAuthDisabled)
		return false
	}
}

func Login(c *gin.Context) {
	var req loginRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}

	user, err := model.GetUserByName(req.Name)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			resp.Fail(c, http.StatusUnauthorized, resp.CodeAuthInvalid, resp.MsgLoginFailed)
			return
		}
		logger.Log.Error("login query failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}
	if !user.CheckPassword(req.Password) {
		resp.Fail(c, http.StatusUnauthorized, resp.CodeAuthInvalid, resp.MsgLoginFailed)
		return
	}
	if !checkUserStatus(c, user) {
		return
	}

	pair, err := model.IssueTokenPair(user)
	if err != nil {
		logger.Log.Error("issue token failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	resp.OK(c, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    pair.ExpiresIn,
		"user":          user,
		"capabilities":  authz.Caps(user.Role, user.VipLevel),
	})
}

func Refresh(c *gin.Context) {
	var req refreshRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}

	pair, err := model.RefreshTokenPair(req.RefreshToken)
	if err != nil {
		if errors.Is(err, model.ErrTokenNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			resp.Fail(c, http.StatusUnauthorized, resp.CodeAuthInvalid, resp.MsgRefreshFailed)
			return
		}
		logger.Log.Error("refresh token failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgRefreshFailed)
		return
	}

	resp.OK(c, pair)
}

func Me(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	user, err := model.GetUserByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			resp.Fail(c, http.StatusUnauthorized, resp.CodeAuthInvalid, resp.MsgAuthInvalid)
			return
		}
		logger.Log.Error("me query failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgAuthInvalid)
		return
	}
	resp.OK(c, gin.H{
		"user":         user,
		"capabilities": authz.Caps(user.Role, user.VipLevel),
	})
}
