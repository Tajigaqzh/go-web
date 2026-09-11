package service

import (
	"errors"
	"fmt"
	"time"

	"go-web/model"
)

var (
	ErrCanvasNotEditable = errors.New("canvas not editable in current status")
	ErrInvalidSubmit     = errors.New("invalid publish submit")
	ErrInvalidAudit      = errors.New("invalid audit action")
)

func CanViewCanvas(canvas *model.Canvas, viewer *model.User) bool {
	if canvas == nil || viewer == nil {
		return false
	}
	if canvas.UserID == viewer.ID || viewer.Role >= model.RoleAuditor {
		return true
	}
	if canvas.PublishStatus != model.CanvasStatusPublished {
		return false
	}
	switch canvas.AccessLevel {
	case model.CanvasAccessPublic:
		return true
	case model.CanvasAccessPublisherVIP:
		minLevel := canvas.RequiredPlanLevel
		if minLevel <= 0 {
			minLevel = 1
		}
		return model.HasActivePublisherSub(canvas.UserID, viewer.ID, minLevel)
	default:
		return false
	}
}

func CanEditCanvas(canvas *model.Canvas, viewer *model.User) bool {
	if canvas == nil || viewer == nil {
		return false
	}
	if viewer.Role >= model.RoleAdmin {
		return true
	}
	return canvas.UserID == viewer.ID
}

// SubmitCanvasForReview 发布者提交审核：免费公开 或 发布者VIP可见。
func SubmitCanvasForReview(canvas *model.Canvas, accessLevel, requiredPlanLevel int) error {
	if canvas == nil {
		return ErrInvalidSubmit
	}
	if canvas.PublishStatus == model.CanvasStatusPending {
		return ErrInvalidSubmit
	}
	if accessLevel != model.CanvasAccessPublic && accessLevel != model.CanvasAccessPublisherVIP {
		return ErrInvalidSubmit
	}
	if accessLevel == model.CanvasAccessPublisherVIP && requiredPlanLevel < 1 {
		requiredPlanLevel = 1
	}

	now := time.Now()
	canvas.AccessLevel = accessLevel
	canvas.RequiredPlanLevel = requiredPlanLevel
	canvas.PublishStatus = model.CanvasStatusPending
	canvas.RejectReason = ""
	canvas.SubmittedAt = &now
	canvas.ReviewedBy = nil
	canvas.ReviewedAt = nil
	if err := canvas.UpdatePublishMeta(); err != nil {
		return err
	}
	return model.NotifyAuditors(
		"画布待审核",
		fmt.Sprintf("画布「%s」(#%d) 已提交审核", canvas.Title, canvas.ID),
		canvas.ID,
	)
}

func ApproveCanvas(canvas *model.Canvas, auditorID int64) error {
	if canvas == nil || canvas.PublishStatus != model.CanvasStatusPending {
		return ErrInvalidAudit
	}
	now := time.Now()
	canvas.PublishStatus = model.CanvasStatusPublished
	canvas.RejectReason = ""
	canvas.ReviewedBy = &auditorID
	canvas.ReviewedAt = &now
	return canvas.UpdatePublishMeta()
}

func RejectCanvas(canvas *model.Canvas, auditorID int64, reason string) error {
	if canvas == nil || canvas.PublishStatus != model.CanvasStatusPending {
		return ErrInvalidAudit
	}
	now := time.Now()
	canvas.PublishStatus = model.CanvasStatusRejected
	canvas.RejectReason = reason
	canvas.ReviewedBy = &auditorID
	canvas.ReviewedAt = &now
	return canvas.UpdatePublishMeta()
}
