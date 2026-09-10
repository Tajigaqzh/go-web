package model

import (
	"fmt"

	"go-web/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB(cfg *config.DatabaseConfig) error {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}

	if err := db.AutoMigrate(
		&User{},
		&Canvas{},
		&Material{},
		&PublisherVipPlan{},
		&PublisherSubscription{},
		&Notification{},
	); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	DB = db
	if err := seedAdmin(); err != nil {
		return err
	}
	if err := seedAuditor(); err != nil {
		return err
	}
	return seedMaterials()
}

func seedAdmin() error {
	var n int64
	if err := DB.Model(&User{}).Where("role = ?", RoleAdmin).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	admin := &User{Name: "admin", Role: RoleAdmin, Status: StatusEnabled}
	if err := admin.SetPassword("admin"); err != nil {
		return err
	}
	return admin.Insert()
}

func seedAuditor() error {
	var n int64
	if err := DB.Model(&User{}).Where("role = ?", RoleAuditor).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	auditor := &User{Name: "auditor", Role: RoleAuditor, Status: StatusEnabled}
	if err := auditor.SetPassword("auditor"); err != nil {
		return err
	}
	return auditor.Insert()
}

func CloseDB() error {
	if DB == nil {
		return nil
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
