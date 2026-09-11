package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go-web/config"
	"go-web/logger"
	"go-web/model"
	"go-web/resp"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type oauthState struct {
	Provider    string `json:"provider"`
	FrontendURL string `json:"frontend_url"`
}

func storeOAuthState(state string, data oauthState) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	val, _ := json.Marshal(data)
	return config.RDB.Set(ctx, "oauth:state:"+state, val, 10*time.Minute).Err()
}

func verifyOAuthState(state string) (*oauthState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	val, err := config.RDB.GetDel(ctx, "oauth:state:"+state).Result()
	if err != nil {
		return nil, err
	}
	var data oauthState
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func getCallbackBaseURL(c *gin.Context) string {
	scheme := "https"
	if c.GetHeader("X-Forwarded-Proto") != "" {
		scheme = c.GetHeader("X-Forwarded-Proto")
	} else if c.Request.TLS == nil {
		scheme = "http"
	}
	return scheme + "://" + c.Request.Host
}

func redirectWithToken(c *gin.Context, frontendURL string, user *model.User) {
	pair, err := model.IssueTokenPair(user)
	if err != nil {
		logger.Log.Error("issue token failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	if frontendURL == "" {
		resp.OK(c, gin.H{
			"access_token":  pair.AccessToken,
			"refresh_token": pair.RefreshToken,
			"expires_in":    pair.ExpiresIn,
		})
		return
	}

	u, _ := url.Parse(frontendURL)
	q := u.Query()
	q.Set("access_token", pair.AccessToken)
	q.Set("refresh_token", pair.RefreshToken)
	q.Set("expires_in", fmt.Sprintf("%d", pair.ExpiresIn))
	u.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, u.String())
}

// ========== GitHub ==========

// GitHubAuthorize 获取 GitHub 授权跳转 URL
func GitHubAuthorize(c *gin.Context) {
	cfg := AuthConfig.GitHub
	if cfg.ClientID == "" {
		resp.Fail(c, http.StatusServiceUnavailable, resp.CodeInternal, "github auth not configured")
		return
	}

	frontendURL := c.Query("frontend_url")
	state := genTicket()
	_ = storeOAuthState(state, oauthState{Provider: "github", FrontendURL: frontendURL})

	redirectURI := cfg.RedirectURI
	if redirectURI == "" {
		redirectURI = getCallbackBaseURL(c) + "/api/v1/oauth/github/callback"
	}

	authURL := fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=user:email&state=%s",
		url.QueryEscape(cfg.ClientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(state),
	)

	resp.OK(c, gin.H{"authorize_url": authURL})
}

type githubAccessTokenResp struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

type githubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
}

func githubExchangeCode(clientID, clientSecret, code, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	req, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)

	var result githubAccessTokenResp
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		return "", errors.New("empty access token from github")
	}
	return result.AccessToken, nil
}

func githubGetUser(accessToken string) (*githubUser, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)

	var user githubUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}
	if user.ID == 0 {
		return nil, errors.New("empty user id from github")
	}
	if user.Email == "" {
		email, err := githubGetPrimaryEmail(accessToken)
		if err == nil {
			user.Email = email
		}
	}
	return &user, nil
}

func githubGetPrimaryEmail(accessToken string) (string, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", err
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	return "", errors.New("no primary verified email")
}

// GitHubCallback GitHub OAuth 回调
func GitHubCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidParam, resp.MsgInvalidParam)
		return
	}

	stateData, err := verifyOAuthState(state)
	if err != nil {
		resp.Fail(c, http.StatusBadRequest, resp.CodeAuthInvalid, "invalid or expired state")
		return
	}

	cfg := AuthConfig.GitHub
	redirectURI := cfg.RedirectURI
	if redirectURI == "" {
		redirectURI = getCallbackBaseURL(c) + "/api/v1/oauth/github/callback"
	}

	accessToken, err := githubExchangeCode(cfg.ClientID, cfg.ClientSecret, code, redirectURI)
	if err != nil {
		logger.Log.Error("github exchange code failed", zap.Error(err))
		resp.Fail(c, http.StatusBadRequest, resp.CodeAuthInvalid, "github authentication failed")
		return
	}

	ghUser, err := githubGetUser(accessToken)
	if err != nil {
		logger.Log.Error("github get user failed", zap.Error(err))
		resp.Fail(c, http.StatusBadRequest, resp.CodeAuthInvalid, "github authentication failed")
		return
	}

	githubID := fmt.Sprintf("github_%d", ghUser.ID)
	user, err := model.GetUserByGitHubID(githubID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			name := ghUser.Login
			if name == "" {
				name = "gh_" + githubID
			}
			if len(name) > 64 {
				name = name[:64]
			}
			user = &model.User{
				Name:     name,
				Email:    ghUser.Email,
				GitHubID: githubID,
				Role:     model.RoleUser,
				VipLevel: model.VipFree,
				Status:   model.StatusEnabled,
			}
			if err := user.Insert(); err != nil {
				logger.Log.Error("github auto register failed", zap.Error(err))
				resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
				return
			}
		} else {
			logger.Log.Error("github login query failed", zap.Error(err))
			resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
			return
		}
	}

	if !checkUserStatus(c, user) {
		return
	}

	redirectWithToken(c, stateData.FrontendURL, user)
}

// ========== Google ==========

// GoogleAuthorize 获取 Google 授权跳转 URL
func GoogleAuthorize(c *gin.Context) {
	cfg := AuthConfig.Google
	if cfg.ClientID == "" {
		resp.Fail(c, http.StatusServiceUnavailable, resp.CodeInternal, "google auth not configured")
		return
	}

	frontendURL := c.Query("frontend_url")
	state := genTicket()
	_ = storeOAuthState(state, oauthState{Provider: "google", FrontendURL: frontendURL})

	redirectURI := cfg.RedirectURI
	if redirectURI == "" {
		redirectURI = getCallbackBaseURL(c) + "/api/v1/oauth/google/callback"
	}

	authURL := fmt.Sprintf("https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=openid+email+profile&state=%s",
		url.QueryEscape(cfg.ClientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(state),
	)

	resp.OK(c, gin.H{"authorize_url": authURL})
}

type googleTokenResp struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type googleUser struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func googleExchangeCode(clientID, clientSecret, code, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")

	req, _ := http.NewRequest("POST", "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)

	var result googleTokenResp
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		return "", errors.New("empty access token from google")
	}
	return result.AccessToken, nil
}

func googleGetUser(accessToken string) (*googleUser, error) {
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)

	var user googleUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}
	if user.ID == "" {
		return nil, errors.New("empty user id from google")
	}
	return &user, nil
}

// GoogleCallback Google OAuth 回调
func GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidParam, resp.MsgInvalidParam)
		return
	}

	stateData, err := verifyOAuthState(state)
	if err != nil {
		resp.Fail(c, http.StatusBadRequest, resp.CodeAuthInvalid, "invalid or expired state")
		return
	}

	cfg := AuthConfig.Google
	redirectURI := cfg.RedirectURI
	if redirectURI == "" {
		redirectURI = getCallbackBaseURL(c) + "/api/v1/oauth/google/callback"
	}

	accessToken, err := googleExchangeCode(cfg.ClientID, cfg.ClientSecret, code, redirectURI)
	if err != nil {
		logger.Log.Error("google exchange code failed", zap.Error(err))
		resp.Fail(c, http.StatusBadRequest, resp.CodeAuthInvalid, "google authentication failed")
		return
	}

	gUser, err := googleGetUser(accessToken)
	if err != nil {
		logger.Log.Error("google get user failed", zap.Error(err))
		resp.Fail(c, http.StatusBadRequest, resp.CodeAuthInvalid, "google authentication failed")
		return
	}

	googleID := "google_" + gUser.ID
	user, err := model.GetUserByGoogleID(googleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			name := gUser.Name
			if name == "" {
				name = "gg_" + gUser.ID
			}
			if len(name) > 64 {
				name = name[:64]
			}
			user = &model.User{
				Name:     name,
				Email:    gUser.Email,
				GoogleID: googleID,
				Role:     model.RoleUser,
				VipLevel: model.VipFree,
				Status:   model.StatusEnabled,
			}
			if err := user.Insert(); err != nil {
				logger.Log.Error("google auto register failed", zap.Error(err))
				resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
				return
			}
		} else {
			logger.Log.Error("google login query failed", zap.Error(err))
			resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
			return
		}
	}

	if !checkUserStatus(c, user) {
		return
	}

	redirectWithToken(c, stateData.FrontendURL, user)
}

// ========== 微信网页授权 ==========

// WechatAuthorize 获取微信网页授权跳转 URL
func WechatAuthorize(c *gin.Context) {
	cfg := AuthConfig.Wechat
	if cfg.AppID == "" {
		resp.Fail(c, http.StatusServiceUnavailable, resp.CodeInternal, "wechat auth not configured")
		return
	}

	frontendURL := c.Query("frontend_url")
	state := genTicket()
	_ = storeOAuthState(state, oauthState{Provider: "wechat", FrontendURL: frontendURL})

	redirectURI := cfg.RedirectURI
	if redirectURI == "" {
		redirectURI = getCallbackBaseURL(c) + "/api/v1/oauth/wechat/callback"
	}

	authURL := fmt.Sprintf("https://open.weixin.qq.com/connect/qrconnect?appid=%s&redirect_uri=%s&response_type=code&scope=snsapi_login&state=%s#wechat_redirect",
		url.QueryEscape(cfg.AppID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(state),
	)

	resp.OK(c, gin.H{"authorize_url": authURL})
}

type wechatUserInfo struct {
	OpenID     string `json:"openid"`
	UnionID    string `json:"unionid"`
	Nickname   string `json:"nickname"`
	HeadImgURL string `json:"headimgurl"`
}

func wechatGetUserInfo(accessToken, openID string) (*wechatUserInfo, error) {
	url := fmt.Sprintf("https://api.weixin.qq.com/sns/userinfo?access_token=%s&openid=%s", accessToken, openID)
	r, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)

	var user wechatUserInfo
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, err
	}
	if user.OpenID == "" {
		return nil, errors.New("empty openid from wechat userinfo")
	}
	return &user, nil
}

// WechatCallback 微信网页授权回调
func WechatCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidParam, resp.MsgInvalidParam)
		return
	}

	stateData, err := verifyOAuthState(state)
	if err != nil {
		resp.Fail(c, http.StatusBadRequest, resp.CodeAuthInvalid, "invalid or expired state")
		return
	}

	cfg := AuthConfig.Wechat
	openID, unionID, err := wechatCode2Session(cfg.AppID, cfg.AppSecret, code)
	if err != nil {
		logger.Log.Error("wechat code2session failed", zap.Error(err))
		resp.Fail(c, http.StatusBadRequest, resp.CodeWechatAuthFailed, resp.MsgWechatAuthFailed)
		return
	}

	user, err := model.GetUserByWechatOpenID(openID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 尝试获取用户信息用于昵称
			wxUser, _ := wechatGetUserInfo(openID, openID)
			name := "wx_" + openID
			if wxUser != nil && wxUser.Nickname != "" {
				name = wxUser.Nickname
				if len(name) > 64 {
					name = name[:64]
				}
			}
			user = &model.User{
				Name:          name,
				WechatOpenID:  openID,
				WechatUnionID: unionID,
				Role:          model.RoleUser,
				VipLevel:      model.VipFree,
				Status:        model.StatusEnabled,
			}
			if err := user.Insert(); err != nil {
				logger.Log.Error("wechat auto register failed", zap.Error(err))
				resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
				return
			}
		} else {
			logger.Log.Error("wechat login query failed", zap.Error(err))
			resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
			return
		}
	}

	if !checkUserStatus(c, user) {
		return
	}

	redirectWithToken(c, stateData.FrontendURL, user)
}
