package model

import (
	"encoding/json"
	"time"
)

const (
	// 发布状态
	CanvasStatusDraft     = 0
	CanvasStatusPending   = 1
	CanvasStatusPublished = 2
	CanvasStatusRejected  = 3

	// 可见范围（仅 published 后对使用者生效）
	CanvasAccessPrivate      = 0 // 仅作者
	CanvasAccessPublic       = 1 // 免费公开给所有登录用户
	CanvasAccessPublisherVIP = 2 // 仅订阅了发布者 VIP 的用户
)

type Canvas struct {
	ID                uint            `json:"id" gorm:"primaryKey" example:"1"`                                  // 画布 ID
	UserID            uint            `json:"user_id" gorm:"index;not null" example:"10"`                        // 所属用户 ID
	Title             string          `json:"title" gorm:"size:128;not null" example:"我的画布"`                     // 画布标题
	Document          json.RawMessage `json:"document" gorm:"type:json;not null" swaggertype:"object"`           // 画布文档内容
	AccessLevel       int             `json:"access_level" gorm:"default:0" enums:"0,1,2" example:"0"`           // 访问级别：0 私有，1 公开，2 仅发布者 VIP 可见
	RequiredPlanLevel int             `json:"required_plan_level" gorm:"default:1" minimum:"1" maximum:"10"`     // 要求的发布者 VIP 等级
	PublishStatus     int             `json:"publish_status" gorm:"default:0;index" enums:"0,1,2,3" example:"0"` // 发布状态：0 草稿，1 待审核，2 已发布，3 已驳回
	RejectReason      string          `json:"reject_reason" gorm:"size:512"`                                     // 审核驳回原因
	SubmittedAt       *time.Time      `json:"submitted_at"`                                                      // 提交审核时间
	ReviewedBy        *uint           `json:"reviewed_by"`                                                       // 审核人用户 ID
	ReviewedAt        *time.Time      `json:"reviewed_at"`                                                       // 审核时间
	CreatedAt         time.Time       `json:"created_at"`                                                        // 创建时间
	UpdatedAt         time.Time       `json:"updated_at"`                                                        // 最后更新时间
}

type CanvasSummary struct {
	ID            uint      `json:"id" example:"1"`                                 // 画布 ID
	Title         string    `json:"title" example:"我的画布"`                           // 画布标题
	AccessLevel   int       `json:"access_level" enums:"0,1,2" example:"0"`         // 访问级别：0 私有，1 公开，2 仅发布者 VIP 可见
	PublishStatus int       `json:"publish_status" enums:"0,1,2,3" example:"0"`     // 发布状态：0 草稿，1 待审核，2 已发布，3 已驳回
	UpdatedAt     time.Time `json:"updated_at" example:"2026-09-10T12:00:00+08:00"` // 最后更新时间
	CreatedAt     time.Time `json:"created_at" example:"2026-09-10T10:00:00+08:00"` // 创建时间
}

func ListCanvasesByUser(userID uint, page, size int) ([]CanvasSummary, int64, error) {
	var total int64
	if err := DB.Model(&Canvas{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []CanvasSummary
	err := DB.Model(&Canvas{}).
		Select("id", "title", "access_level", "publish_status", "created_at", "updated_at").
		Where("user_id = ?", userID).
		Order("updated_at desc").
		Offset((page - 1) * size).
		Limit(size).
		Find(&list).Error
	return list, total, err
}

func ListPendingCanvases(page, size int) ([]Canvas, int64, error) {
	var total int64
	q := DB.Model(&Canvas{}).Where("publish_status = ?", CanvasStatusPending)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Canvas
	err := q.Order("submitted_at asc").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

func GetCanvasByID(id uint) (*Canvas, error) {
	var canvas Canvas
	if err := DB.First(&canvas, id).Error; err != nil {
		return nil, err
	}
	return &canvas, nil
}

func (c *Canvas) Insert() error {
	return DB.Create(c).Error
}

func (c *Canvas) UpdateContent() error {
	return DB.Model(c).Select("title", "document", "updated_at").Updates(c).Error
}

func (c *Canvas) UpdatePublishMeta() error {
	return DB.Model(c).Select(
		"access_level", "required_plan_level", "publish_status", "reject_reason",
		"submitted_at", "reviewed_by", "reviewed_at", "updated_at",
	).Updates(c).Error
}

func DeleteCanvas(id, userID uint) error {
	return DB.Where("id = ? AND user_id = ?", id, userID).Delete(&Canvas{}).Error
}
