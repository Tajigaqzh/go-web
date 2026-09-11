package model

import "time"

const (
	NotifyTypeCanvasReview = "canvas_review"
)

type Notification struct {
	ID        int64      `json:"id" gorm:"primaryKey;autoIncrement:false"`
	UserID    int64      `json:"user_id" gorm:"index;not null"`
	Type      string     `json:"type" gorm:"size:32;index"`
	Title     string     `json:"title" gorm:"size:128"`
	Content   string     `json:"content" gorm:"size:512"`
	RefType   string     `json:"ref_type" gorm:"size:32"`
	RefID     int64      `json:"ref_id"`
	Read      bool       `json:"read" gorm:"default:false"`
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at"`
}

func (n *Notification) Insert() error {
	if n.ID == 0 {
		n.ID = NextID()
	}
	return DB.Create(n).Error
}

func ListNotifications(userID int64, page, size int) ([]Notification, int64, error) {
	var total int64
	q := DB.Model(&Notification{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []Notification
	err := q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

func MarkNotificationRead(id, userID int64) error {
	now := time.Now()
	return DB.Model(&Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{"read": true, "read_at": now}).Error
}

func ListAuditorIDs() ([]int64, error) {
	var ids []int64
	err := DB.Model(&User{}).
		Where("role = ? AND status = ?", RoleAuditor, StatusEnabled).
		Pluck("id", &ids).Error
	return ids, err
}

func NotifyAuditors(title, content string, canvasID int64) error {
	ids, err := ListAuditorIDs()
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		// 没有专职审核员时通知管理员
		err = DB.Model(&User{}).
			Where("role = ? AND status = ?", RoleAdmin, StatusEnabled).
			Pluck("id", &ids).Error
		if err != nil {
			return err
		}
	}
	for _, id := range ids {
		n := &Notification{
			UserID:  id,
			Type:    NotifyTypeCanvasReview,
			Title:   title,
			Content: content,
			RefType: "canvas",
			RefID:   canvasID,
		}
		if err := n.Insert(); err != nil {
			return err
		}
	}
	return nil
}
