package model

import (
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID            int64  `json:"id" gorm:"primaryKey;autoIncrement:false"`
	Name          string `json:"name" gorm:"uniqueIndex;size:64"`
	PasswordHash  string `json:"-"`
	Phone         string `json:"phone" gorm:"index;size:20"`
	Email         string `json:"email" gorm:"index;size:128"`
	WechatOpenID  string `json:"-" gorm:"index;size:64"`
	WechatUnionID string `json:"-" gorm:"size:64"`
	GitHubID      string `json:"-" gorm:"index;size:64"`
	GoogleID      string `json:"-" gorm:"index;size:64"`
	Role          int    `json:"role"`
	VipLevel      int    `json:"vip_level"`
	Status        int    `json:"status"`
}

func ListUsers(page, size int) ([]User, int64, error) {
	var users []User
	var total int64
	q := DB.Model(&User{}).Where("status != ?", StatusDeleted)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Offset((page - 1) * size).Limit(size).Find(&users).Error
	return users, total, err
}

func GetUserByID(id int64) (*User, error) {
	var user User
	if err := DB.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserByName(name string) (*User, error) {
	var user User
	if err := DB.Where("name = ?", name).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserByPhone(phone string) (*User, error) {
	var user User
	if err := DB.Where("phone = ?", phone).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserByEmail(email string) (*User, error) {
	var user User
	if err := DB.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserByWechatOpenID(openID string) (*User, error) {
	var user User
	if err := DB.Where("wechat_open_id = ?", openID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserByGitHubID(githubID string) (*User, error) {
	var user User
	if err := DB.Where("github_id = ?", githubID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserByGoogleID(googleID string) (*User, error) {
	var user User
	if err := DB.Where("google_id = ?", googleID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (user *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hash)
	return nil
}

func (user *User) CheckPassword(password string) bool {
	if user.PasswordHash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) == nil
}

func (user *User) Insert() error {
	if user.ID == 0 {
		user.ID = NextID()
	}
	if user.Role == 0 {
		user.Role = RoleUser
	}
	if user.Status == 0 {
		user.Status = StatusEnabled
	}
	return DB.Create(user).Error
}

func (user *User) UpdateStatus(status int) error {
	return DB.Model(user).Update("status", status).Error
}
