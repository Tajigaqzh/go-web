package model

import "time"

// PublisherVipPlan 发布者自建 VIP 档位。
type PublisherVipPlan struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	PublisherID uint      `json:"publisher_id" gorm:"index;not null"`
	Name        string    `json:"name" gorm:"size:64;not null"`
	Level       int       `json:"level" gorm:"not null"` // 1,2,...
	Description string    `json:"description" gorm:"size:256"`
	Enabled     bool      `json:"enabled" gorm:"default:true"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func ListPublisherVipPlans(publisherID uint) ([]PublisherVipPlan, error) {
	var list []PublisherVipPlan
	err := DB.Where("publisher_id = ? AND enabled = ?", publisherID, true).
		Order("level asc").Find(&list).Error
	return list, err
}

func GetPublisherVipPlan(id uint) (*PublisherVipPlan, error) {
	var plan PublisherVipPlan
	if err := DB.First(&plan, id).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

func (p *PublisherVipPlan) Insert() error {
	return DB.Create(p).Error
}

// PublisherSubscription 用户订阅某发布者的 VIP。
type PublisherSubscription struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	PublisherID  uint      `json:"publisher_id" gorm:"uniqueIndex:idx_pub_sub;not null"`
	SubscriberID uint      `json:"subscriber_id" gorm:"uniqueIndex:idx_pub_sub;not null"`
	PlanID       uint      `json:"plan_id" gorm:"index;not null"`
	Level        int       `json:"level"`
	ExpireAt     time.Time `json:"expire_at"`
	Status       int       `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func HasActivePublisherSub(publisherID, subscriberID uint, minLevel int) bool {
	if publisherID == 0 || subscriberID == 0 {
		return false
	}
	var n int64
	err := DB.Model(&PublisherSubscription{}).
		Where("publisher_id = ? AND subscriber_id = ? AND status = ? AND level >= ? AND expire_at > ?",
			publisherID, subscriberID, StatusEnabled, minLevel, time.Now()).
		Count(&n).Error
	return err == nil && n > 0
}

func (s *PublisherSubscription) Insert() error {
	if s.Status == 0 {
		s.Status = StatusEnabled
	}
	return DB.Create(s).Error
}
