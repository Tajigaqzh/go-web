package config

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var RDB *redis.Client

func InitRedis(cfg *RedisConfig) error {
	RDB = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := RDB.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect redis: %w", err)
	}
	return nil
}

func CloseRedis() error {
	if RDB == nil {
		return nil
	}
	return RDB.Close()
}

func refreshKey(token string) string {
	return "refresh:" + token
}

func SetRefreshToken(token string, userID uint, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return RDB.Set(ctx, refreshKey(token), userID, ttl).Err()
}

func GetRefreshUserID(token string) (uint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	val, err := RDB.Get(ctx, refreshKey(token)).Result()
	if err != nil {
		return 0, err
	}
	id, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

func DeleteRefreshToken(token string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return RDB.Del(ctx, refreshKey(token)).Err()
}
