package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"go-web/logger"
	"go-web/middleware"
	"go-web/model"
	"go-web/resp"
	"go-web/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type listCanvasQuery struct {
	Page int `form:"page" binding:"omitempty,min=1" example:"1"`          // 页码，从 1 开始
	Size int `form:"size" binding:"omitempty,min=1,max=100" example:"20"` // 每页记录数，最大 100
}

type saveCanvasRequest struct {
	Title    string          `json:"title" binding:"required,min=1,max=128" example:"我的画布"` // 画布标题
	Document json.RawMessage `json:"document" binding:"required" swaggertype:"object"`      // 画布文档内容，必须是合法的 JSON 对象
}

type submitCanvasRequest struct {
	AccessLevel       int `json:"access_level" binding:"required,oneof=1 2" enums:"1,2" example:"1"`             // 访问级别：1 公开，2 仅发布者 VIP 可见
	RequiredPlanLevel int `json:"required_plan_level" binding:"omitempty,min=1,max=10" minimum:"1" maximum:"10"` // access_level 为 2 时要求的 VIP 等级
}

// ListCanvases godoc
// @Summary 获取画布列表
// @Tags canvases
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码，从 1 开始" minimum(1)
// @Param size query int false "每页记录数" minimum(1) maximum(100)
// @Success 200 {object} resp.Response{data=[]model.CanvasSummary}
// @Failure 400,401,403,500 {object} resp.Response
// @Router /canvases [get]
func ListCanvases(c *gin.Context) {
	var q listCanvasQuery
	if err := resp.BindQuery(c, &q); err != nil {
		return
	}
	if q.Page == 0 {
		q.Page = 1
	}
	if q.Size == 0 {
		q.Size = 20
	}

	list, total, err := model.ListCanvasesByUser(middleware.CurrentUserID(c), q.Page, q.Size)
	if err != nil {
		logger.Log.Error("list canvases failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgListCanvasFailed)
		return
	}
	resp.OKWithPagination(c, list, resp.Pagination{
		Page:  q.Page,
		Size:  q.Size,
		Total: int(total),
	})
}

// GetCanvas godoc
// @Summary 获取画布详情
// @Tags canvases
// @Produce json
// @Security BearerAuth
// @Param id path int true "画布 ID"
// @Success 200 {object} resp.Response{data=model.Canvas}
// @Failure 400,401,403,404,500 {object} resp.Response
// @Router /canvases/{id} [get]
func GetCanvas(c *gin.Context) {
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
		logger.Log.Error("get canvas failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgGetCanvasFailed)
		return
	}

	viewer, err := model.GetUserByID(middleware.CurrentUserID(c))
	if err != nil {
		resp.Fail(c, http.StatusUnauthorized, resp.CodeAuthInvalid, resp.MsgAuthInvalid)
		return
	}
	if !service.CanViewCanvas(canvas, viewer) {
		resp.Fail(c, http.StatusForbidden, resp.CodeAuthForbidden, resp.MsgCanvasAccessDenied)
		return
	}
	resp.OK(c, canvas)
}

// CreateCanvas godoc
// @Summary 创建画布
// @Tags canvases
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body saveCanvasRequest true "画布内容"
// @Success 201 {object} resp.Response{data=model.Canvas}
// @Failure 400,401,403,500 {object} resp.Response
// @Router /canvases [post]
func CreateCanvas(c *gin.Context) {
	var req saveCanvasRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}
	if !json.Valid(req.Document) {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidBody, resp.MsgInvalidDocument)
		return
	}

	canvas := &model.Canvas{
		UserID:        middleware.CurrentUserID(c),
		Title:         req.Title,
		Document:      req.Document,
		AccessLevel:   model.CanvasAccessPrivate,
		PublishStatus: model.CanvasStatusDraft,
	}
	if err := canvas.Insert(); err != nil {
		logger.Log.Error("create canvas failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgSaveCanvasFailed)
		return
	}
	resp.Created(c, canvas)
}

// UpdateCanvas godoc
// @Summary 更新画布
// @Tags canvases
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "画布 ID"
// @Param request body saveCanvasRequest true "画布内容"
// @Success 200 {object} resp.Response{data=model.Canvas}
// @Failure 400,401,403,404,409,500 {object} resp.Response
// @Router /canvases/updateById/{id} [post]
func UpdateCanvas(c *gin.Context) {
	id, err := parseCanvasID(c)
	if err != nil {
		return
	}
	var req saveCanvasRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}
	if !json.Valid(req.Document) {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidBody, resp.MsgInvalidDocument)
		return
	}

	canvas, viewer, ok := loadOwnedCanvas(c, id)
	if !ok {
		return
	}
	if !service.CanEditCanvas(canvas, viewer) {
		resp.Fail(c, http.StatusForbidden, resp.CodeAuthForbidden, resp.MsgAuthForbidden)
		return
	}
	if canvas.PublishStatus == model.CanvasStatusPending {
		resp.Fail(c, http.StatusConflict, resp.CodeConflict, resp.MsgCanvasPending)
		return
	}

	canvas.Title = req.Title
	canvas.Document = req.Document
	// 已发布内容再次编辑后回到草稿，需重新提交审核
	if canvas.PublishStatus == model.CanvasStatusPublished || canvas.PublishStatus == model.CanvasStatusRejected {
		canvas.PublishStatus = model.CanvasStatusDraft
		_ = canvas.UpdatePublishMeta()
	}
	if err := canvas.UpdateContent(); err != nil {
		logger.Log.Error("update canvas failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgSaveCanvasFailed)
		return
	}
	resp.OK(c, canvas)
}

// DeleteCanvas godoc
// @Summary 删除画布
// @Tags canvases
// @Produce json
// @Security BearerAuth
// @Param id path int true "画布 ID"
// @Success 200 {object} resp.Response
// @Failure 400,401,403,404,500 {object} resp.Response
// @Router /canvases/deleteById/{id} [post]
func DeleteCanvas(c *gin.Context) {
	id, err := parseCanvasID(c)
	if err != nil {
		return
	}
	canvas, viewer, ok := loadOwnedCanvas(c, id)
	if !ok {
		return
	}
	if !service.CanEditCanvas(canvas, viewer) {
		resp.Fail(c, http.StatusForbidden, resp.CodeAuthForbidden, resp.MsgAuthForbidden)
		return
	}
	if err := model.DeleteCanvas(id, middleware.CurrentUserID(c)); err != nil {
		logger.Log.Error("delete canvas failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgDeleteCanvasFailed)
		return
	}
	resp.OK(c)
}

// SubmitCanvas godoc
// @Summary 提交画布审核
// @Tags canvases
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "画布 ID"
// @Param request body submitCanvasRequest true "发布设置"
// @Success 200 {object} resp.Response{data=model.Canvas}
// @Failure 400,401,403,404,500 {object} resp.Response
// @Router /canvases/{id}/submit [post]
func SubmitCanvas(c *gin.Context) {
	id, err := parseCanvasID(c)
	if err != nil {
		return
	}
	var req submitCanvasRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}
	canvas, viewer, ok := loadOwnedCanvas(c, id)
	if !ok {
		return
	}
	if !service.CanEditCanvas(canvas, viewer) {
		resp.Fail(c, http.StatusForbidden, resp.CodeAuthForbidden, resp.MsgAuthForbidden)
		return
	}
	if err := service.SubmitCanvasForReview(canvas, req.AccessLevel, req.RequiredPlanLevel); err != nil {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidParam, resp.MsgSubmitCanvasFailed)
		return
	}
	resp.OK(c, canvas)
}

func parseCanvasID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidParam, resp.MsgInvalidCanvasID)
		return 0, err
	}
	return uint(id), nil
}

func loadOwnedCanvas(c *gin.Context, id uint) (*model.Canvas, *model.User, bool) {
	canvas, err := model.GetCanvasByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			resp.Fail(c, http.StatusNotFound, resp.CodeNotFound, resp.MsgCanvasNotFound)
		} else {
			resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgGetCanvasFailed)
		}
		return nil, nil, false
	}
	viewer, err := model.GetUserByID(middleware.CurrentUserID(c))
	if err != nil {
		resp.Fail(c, http.StatusUnauthorized, resp.CodeAuthInvalid, resp.MsgAuthInvalid)
		return nil, nil, false
	}
	return canvas, viewer, true
}
