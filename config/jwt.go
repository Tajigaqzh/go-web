package config

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	jwtSecret  []byte
	accessTTL  = 2 * time.Hour
	refreshTTL = 168 * time.Hour
)

type Claims struct {
	UserID   uint `json:"user_id"`
	Role     int  `json:"role"`
	VipLevel int  `json:"vip_level"`
	jwt.RegisteredClaims
}

func InitJWT(cfg *JWTConfig) {
	jwtSecret = []byte(cfg.Secret)
	accessTTL = cfg.AccessDuration()
	refreshTTL = cfg.RefreshDuration()
}

func AccessTTL() time.Duration {
	return accessTTL
}

func RefreshTTL() time.Duration {
	return refreshTTL
}

func IssueAccessToken(userID uint, role, vipLevel int) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Role:     role,
		VipLevel: vipLevel,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func ParseAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
