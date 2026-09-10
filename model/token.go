package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"go-web/config"

	"github.com/redis/go-redis/v9"
)

var ErrTokenNotFound = errors.New("token not found")

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func IssueTokenPair(user *User) (*TokenPair, error) {
	access, err := config.IssueAccessToken(user.ID, user.Role, user.VipLevel)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	refresh := hex.EncodeToString(buf)
	if err := config.SetRefreshToken(refresh, user.ID, config.RefreshTTL()); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(config.AccessTTL().Seconds()),
	}, nil
}

func RefreshTokenPair(refreshToken string) (*TokenPair, error) {
	userID, err := config.GetRefreshUserID(refreshToken)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}

	user, err := GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	if user.Status != StatusEnabled {
		_ = config.DeleteRefreshToken(refreshToken)
		return nil, ErrTokenNotFound
	}

	_ = config.DeleteRefreshToken(refreshToken)
	return IssueTokenPair(user)
}
