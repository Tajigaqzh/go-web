package controller

import (
	"net/http"
	"time"

	"go-web/logger"
	"go-web/middleware"
	"go-web/model"
	"go-web/resp"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type createVipPlanRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=64"`
	Level       int    `json:"level" binding:"required,min=1,max=10"`
	Description string `json:"description" binding:"omitempty,max=256"`
}

type subscribeRequest struct {
	PlanID int64 `json:"plan_id" binding:"required"`
	Days   int   `json:"days" binding:"omitempty,min=1,max=365"`
}

func ListMyVipPlans(c *gin.Context) {
	list, err := model.ListPublisherVipPlans(middleware.CurrentUserID(c))
	if err != nil {
		logger.Log.Error("list vip plans failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgListVipPlanFailed)
		return
	}
	resp.OK(c, list)
}

func CreateVipPlan(c *gin.Context) {
	var req createVipPlanRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}
	plan := &model.PublisherVipPlan{
		PublisherID: middleware.CurrentUserID(c),
		Name:        req.Name,
		Level:       req.Level,
		Description: req.Description,
		Enabled:     true,
	}
	if err := plan.Insert(); err != nil {
		logger.Log.Error("create vip plan failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgSaveVipPlanFailed)
		return
	}
	resp.Created(c, plan)
}

func ListPublisherVipPlans(c *gin.Context) {
	publisherID, err := parseIDParam(c)
	if err != nil {
		return
	}
	list, err := model.ListPublisherVipPlans(publisherID)
	if err != nil {
		logger.Log.Error("list publisher vip plans failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgListVipPlanFailed)
		return
	}
	resp.OK(c, list)
}

func SubscribePublisherVip(c *gin.Context) {
	publisherID, err := parseIDParam(c)
	if err != nil {
		return
	}
	var req subscribeRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}
	plan, err := model.GetPublisherVipPlan(req.PlanID)
	if err != nil || plan.PublisherID != publisherID || !plan.Enabled {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidParam, resp.MsgInvalidVipPlan)
		return
	}
	days := req.Days
	if days == 0 {
		days = 30
	}
	sub := &model.PublisherSubscription{
		PublisherID:  publisherID,
		SubscriberID: middleware.CurrentUserID(c),
		PlanID:       plan.ID,
		Level:        plan.Level,
		ExpireAt:     time.Now().AddDate(0, 0, days),
		Status:       model.StatusEnabled,
	}
	if err := sub.Insert(); err != nil {
		logger.Log.Error("subscribe publisher vip failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgSubscribeFailed)
		return
	}
	resp.Created(c, sub)
}
