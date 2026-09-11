package model

import "time"

// PublisherVipPlan 发布者自建 VIP 档位。
type PublisherVipPlan struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement:false"`
	PublisherID int64     `json:"publisher_id" gorm:"index;not null"`
	Name        string    `json:"name" gorm:"size:64;not null"`
	Level       int       `json:"level" gorm:"not null"` // 1,2,...
	Description string    `json:"description" gorm:"size:256"`
	Enabled     bool      `json:"enabled" gorm:"default:true"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func ListPublisherVipPlans(publisherID int64) ([]PublisherVipPlan, error) {
	var list []PublisherVipPlan
	err := DB.Where("publisher_id = ? AND enabled = ?", publisherID, true).
		Order("level asc").Find(&list).Error
	return list, err
}

func GetPublisherVipPlan(id int64) (*PublisherVipPlan, error) {
	var plan PublisherVipPlan
	if err := DB.First(&plan, id).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

func (p *PublisherVipPlan) Insert() error {
	if p.ID == 0 {
		p.ID = NextID()
	}
	return DB.Create(p).Error
}

// PublisherSubscription 用户订阅某发布者的 VIP。
type PublisherSubscription struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement:false"`
	PublisherID  int64     `json:"publisher_id" gorm:"uniqueIndex:idx_pub_sub;not null"`
	SubscriberID int64     `json:"subscriber_id" gorm:"uniqueIndex:idx_pub_sub;not null"`
	PlanID       int64     `json:"plan_id" gorm:"index;not null"`
	Level        int       `json:"level"`
	ExpireAt     time.Time `json:"expire_at"`
	Status       int       `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func HasActivePublisherSub(publisherID, subscriberID int64, minLevel int) bool {
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
	if s.ID == 0 {
		s.ID = NextID()
	}
	if s.Status == 0 {
		s.Status = StatusEnabled
	}
	return DB.Create(s).Error
}
