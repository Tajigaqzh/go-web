package controller

import (
	"errors"
	"net/http"

	"go-web/logger"
	"go-web/middleware"
	"go-web/model"
	"go-web/resp"
	"go-web/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type listAuditQuery struct {
	Page int `form:"page" binding:"omitempty,min=1"`
	Size int `form:"size" binding:"omitempty,min=1,max=100"`
}

type rejectRequest struct {
	Reason string `json:"reason" binding:"required,min=1,max=512"`
}

func ListPendingCanvases(c *gin.Context) {
	var q listAuditQuery
	if err := resp.BindQuery(c, &q); err != nil {
		return
	}
	if q.Page == 0 {
		q.Page = 1
	}
	if q.Size == 0 {
		q.Size = 20
	}
	list, total, err := model.ListPendingCanvases(q.Page, q.Size)
	if err != nil {
		logger.Log.Error("list pending canvases failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgListCanvasFailed)
		return
	}
	resp.OKWithPagination(c, list, resp.Pagination{Page: q.Page, Size: q.Size, Total: int(total)})
}

func ApproveCanvas(c *gin.Context) {
	id, err := parseCanvasID(c)
	if err != nil {
		return
	}
	canvas, err := model.GetCanvasByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			resp.Fail(c, http.StatusNotFound, resp.CodeNotFound, resp.MsgCanvasNotFound)
			return
		}
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgGetCanvasFailed)
		return
	}
	if err := service.ApproveCanvas(canvas, middleware.CurrentUserID(c)); err != nil {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidParam, resp.MsgAuditFailed)
		return
	}
	resp.OK(c, canvas)
}

func RejectCanvas(c *gin.Context) {
	id, err := parseCanvasID(c)
	if err != nil {
		return
	}
	var req rejectRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}
	canvas, err := model.GetCanvasByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			resp.Fail(c, http.StatusNotFound, resp.CodeNotFound, resp.MsgCanvasNotFound)
			return
		}
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgGetCanvasFailed)
		return
	}
	if err := service.RejectCanvas(canvas, middleware.CurrentUserID(c), req.Reason); err != nil {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidParam, resp.MsgAuditFailed)
		return
	}
	resp.OK(c, canvas)
}
