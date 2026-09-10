package model

import (
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID           uint   `json:"id" gorm:"primaryKey"`
	Name         string `json:"name" gorm:"uniqueIndex;size:64"`
	PasswordHash string `json:"-"`
	Role         int    `json:"role"`
	VipLevel     int    `json:"vip_level"`
	Status       int    `json:"status"`
}

func ListUsers(page, size int) ([]User, int64, error) {
	var users []User
	var total int64
	if err := DB.Model(&User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := DB.Offset((page - 1) * size).Limit(size).Find(&users).Error
	return users, total, err
}

func GetUserByID(id uint) (*User, error) {
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
	if user.Role == 0 {
		user.Role = RoleUser
	}
	if user.Status == 0 {
		user.Status = StatusEnabled
	}
	return DB.Create(user).Error
}
