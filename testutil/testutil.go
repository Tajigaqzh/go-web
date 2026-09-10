package testutil

import (
	"log"

	"go-web/authz"
	"go-web/config"
	"go-web/model"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func SetupTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to test database: %v", err)
	}

	if err := db.AutoMigrate(
		&model.User{},
		&model.Canvas{},
		&model.Material{},
		&model.PublisherVipPlan{},
		&model.PublisherSubscription{},
		&model.Notification{},
	); err != nil {
		log.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func SetupTestAuth() {
	config.InitJWT(&config.JWTConfig{
		Secret:     "test-secret",
		AccessTTL:  "2h",
		RefreshTTL: "168h",
	})
	if err := authz.Init(); err != nil {
		log.Fatalf("Failed to init authz: %v", err)
	}
	mr, err := miniredis.Run()
	if err != nil {
		log.Fatalf("Failed to start miniredis: %v", err)
	}
	config.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func CleanupTestDB(db *gorm.DB) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("Failed to get database instance: %v", err)
		return
	}
	sqlDB.Close()
}
