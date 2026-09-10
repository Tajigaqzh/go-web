package controller

import (
	"net/http"

	"go-web/logger"
	"go-web/middleware"
	"go-web/model"
	"go-web/resp"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type listNotifyQuery struct {
	Page int `form:"page" binding:"omitempty,min=1"`
	Size int `form:"size" binding:"omitempty,min=1,max=100"`
}

func ListNotifications(c *gin.Context) {
	var q listNotifyQuery
	if err := resp.BindQuery(c, &q); err != nil {
		return
	}
	if q.Page == 0 {
		q.Page = 1
	}
	if q.Size == 0 {
		q.Size = 20
	}
	list, total, err := model.ListNotifications(middleware.CurrentUserID(c), q.Page, q.Size)
	if err != nil {
		logger.Log.Error("list notifications failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgListNotifyFailed)
		return
	}
	resp.OKWithPagination(c, list, resp.Pagination{Page: q.Page, Size: q.Size, Total: int(total)})
}

func ReadNotification(c *gin.Context) {
	id, err := parseCanvasID(c)
	if err != nil {
		return
	}
	if err := model.MarkNotificationRead(id, middleware.CurrentUserID(c)); err != nil {
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgListNotifyFailed)
		return
	}
	resp.OK(c)
}
