package controller

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"go-web/authz"
	"go-web/config"
	"go-web/logger"
	"go-web/middleware"
	"go-web/model"
	"go-web/resp"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AuthConfig 由 main.go 注入，避免每次重新读取配置文件
var AuthConfig config.AuthConfig

// ========== 通用辅助函数 ==========

func genCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n.Int64())
}

func storeCode(key, code string, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return config.RDB.Set(ctx, key, code, ttl).Err()
}

func verifyCode(key, code string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	val, err := config.RDB.Get(ctx, key).Result()
	if err != nil {
		return false
	}
	if val != code {
		return false
	}
	_ = config.RDB.Del(ctx, key)
	return true
}

func checkSendLimit(key string, max int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	val, err := config.RDB.Get(ctx, key).Int()
	if err == redis.Nil {
		_ = config.RDB.Set(ctx, key, 1, time.Minute)
		return true
	}
	if err != nil {
		return false
	}
	if val >= max {
		return false
	}
	_ = config.RDB.Incr(ctx, key)
	return true
}

func issueAuthResponse(c *gin.Context, user *model.User) {
	pair, err := model.IssueTokenPair(user)
	if err != nil {
		logger.Log.Error("issue token failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}
	resp.OK(c, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    pair.ExpiresIn,
		"user":          user,
		"capabilities":  authz.Caps(user.Role, user.VipLevel),
	})
}

// ========== 短信验证码登录 ==========

type sendSmsCodeRequest struct {
	Phone string `json:"phone" binding:"required"`
}

type smsLoginRequest struct {
	Phone string `json:"phone" binding:"required"`
	Code  string `json:"code" binding:"required,len=6"`
}

func SendSmsCode(c *gin.Context) {
	var req sendSmsCodeRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}

	if !checkSendLimit("sms:limit:"+req.Phone, 1) {
		resp.Fail(c, http.StatusTooManyRequests, resp.CodeRateLimited, resp.MsgRateLimited)
		return
	}

	code := genCode()
	if err := storeCode("sms:code:"+req.Phone, code, 5*time.Minute); err != nil {
		logger.Log.Error("store sms code failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgSendFailed)
		return
	}

	// TODO: 替换为真实短信服务商（阿里云、腾讯云等）
	logger.Log.Info("sms verification code sent", zap.String("phone", req.Phone), zap.String("code", code))

	resp.OK(c, gin.H{"expires_in": 300})
}

func SmsLogin(c *gin.Context) {
	var req smsLoginRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}

	if !verifyCode("sms:code:"+req.Phone, req.Code) {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidCode, resp.MsgInvalidCode)
		return
	}

	user, err := model.GetUserByPhone(req.Phone)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			resp.Fail(c, http.StatusUnauthorized, resp.CodeAuthInvalid, resp.MsgLoginFailed)
			return
		}
		logger.Log.Error("sms login query failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	if !checkUserStatus(c, user) {
		return
	}

	issueAuthResponse(c, user)
}

type smsRegisterRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Code     string `json:"code" binding:"required,len=6"`
	Password string `json:"password" binding:"required"`
}

func SmsRegister(c *gin.Context) {
	var req smsRegisterRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}

	if !verifyCode("sms:code:"+req.Phone, req.Code) {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidCode, resp.MsgInvalidCode)
		return
	}

	if _, err := model.GetUserByPhone(req.Phone); err == nil {
		resp.Fail(c, http.StatusConflict, resp.CodePhoneExists, resp.MsgPhoneExists)
		return
	}

	plainPassword, err := decryptPassword(req.Password)
	if err != nil {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidBody, "invalid encrypted password")
		return
	}

	user := &model.User{
		Name:     req.Phone,
		Phone:    req.Phone,
		Role:     model.RoleUser,
		VipLevel: model.VipFree,
		Status:   model.StatusEnabled,
	}
	if err := user.SetPassword(plainPassword); err != nil {
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgRegisterFailed)
		return
	}
	if err := user.Insert(); err != nil {
		logger.Log.Error("sms register failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgRegisterFailed)
		return
	}

	resp.Created(c, user)
}

// ========== 邮箱验证码登录 ==========

type sendEmailCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type emailLoginRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
}

func SendEmailCode(c *gin.Context) {
	var req sendEmailCodeRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}

	if !checkSendLimit("email:limit:"+req.Email, 1) {
		resp.Fail(c, http.StatusTooManyRequests, resp.CodeRateLimited, resp.MsgRateLimited)
		return
	}

	code := genCode()
	if err := storeCode("email:code:"+req.Email, code, 5*time.Minute); err != nil {
		logger.Log.Error("store email code failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgSendFailed)
		return
	}

	// TODO: 替换为真实邮件服务商（SMTP、SendGrid 等）
	logger.Log.Info("email verification code sent", zap.String("email", req.Email), zap.String("code", code))

	resp.OK(c, gin.H{"expires_in": 300})
}

func EmailLogin(c *gin.Context) {
	var req emailLoginRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}

	if !verifyCode("email:code:"+req.Email, req.Code) {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidCode, resp.MsgInvalidCode)
		return
	}

	user, err := model.GetUserByEmail(req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			resp.Fail(c, http.StatusUnauthorized, resp.CodeAuthInvalid, resp.MsgLoginFailed)
			return
		}
		logger.Log.Error("email login query failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	if !checkUserStatus(c, user) {
		return
	}

	issueAuthResponse(c, user)
}

type emailRegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Code     string `json:"code" binding:"required,len=6"`
	Password string `json:"password" binding:"required"`
}

func EmailRegister(c *gin.Context) {
	var req emailRegisterRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}

	if !verifyCode("email:code:"+req.Email, req.Code) {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidCode, resp.MsgInvalidCode)
		return
	}

	if _, err := model.GetUserByEmail(req.Email); err == nil {
		resp.Fail(c, http.StatusConflict, resp.CodeEmailExists, resp.MsgEmailExists)
		return
	}

	plainPassword, err := decryptPassword(req.Password)
	if err != nil {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidBody, "invalid encrypted password")
		return
	}

	user := &model.User{
		Name:     req.Email,
		Email:    req.Email,
		Role:     model.RoleUser,
		VipLevel: model.VipFree,
		Status:   model.StatusEnabled,
	}
	if err := user.SetPassword(plainPassword); err != nil {
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgRegisterFailed)
		return
	}
	if err := user.Insert(); err != nil {
		logger.Log.Error("email register failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgRegisterFailed)
		return
	}

	resp.Created(c, user)
}

// ========== 微信登录 ==========

type wechatLoginRequest struct {
	Code string `json:"code" binding:"required"`
}

type wechatSessionResp struct {
	OpenID  string `json:"openid"`
	UnionID string `json:"unionid"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func wechatCode2Session(appID, appSecret, code string) (openID, unionID string, err error) {
	url := fmt.Sprintf("https://api.weixin.qq.com/sns/oauth2/access_token?appid=%s&secret=%s&code=%s&grant_type=authorization_code",
		appID, appSecret, code)
	r, err := http.Get(url)
	if err != nil {
		return "", "", err
	}
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)

	var result wechatSessionResp
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", err
	}
	if result.ErrCode != 0 {
		return "", "", fmt.Errorf("wechat error %d: %s", result.ErrCode, result.ErrMsg)
	}
	if result.OpenID == "" {
		return "", "", errors.New("empty openid from wechat")
	}
	return result.OpenID, result.UnionID, nil
}

func WechatLogin(c *gin.Context) {
	var req wechatLoginRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}

	cfg := AuthConfig.Wechat
	if cfg.AppID == "" || cfg.AppSecret == "" {
		resp.Fail(c, http.StatusServiceUnavailable, resp.CodeInternal, "wechat auth not configured")
		return
	}

	openID, unionID, err := wechatCode2Session(cfg.AppID, cfg.AppSecret, req.Code)
	if err != nil {
		logger.Log.Error("wechat code2session failed", zap.Error(err))
		resp.Fail(c, http.StatusBadRequest, resp.CodeWechatAuthFailed, resp.MsgWechatAuthFailed)
		return
	}

	user, err := model.GetUserByWechatOpenID(openID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			name := "wx_" + openID
			if len(name) > 64 {
				name = name[:64]
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

	if user.Status != model.StatusEnabled {
		resp.Fail(c, http.StatusForbidden, resp.CodeAuthDisabled, resp.MsgAuthDisabled)
		return
	}

	issueAuthResponse(c, user)
}

// ========== 扫码登录 ==========

type qrcodeData struct {
	Status string `json:"status"` // pending, scanned, confirmed
	UserID int64  `json:"user_id"`
}

func genTicket() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GetQrcode 生成扫码登录 ticket
func GetQrcode(c *gin.Context) {
	ticket := genTicket()
	data := qrcodeData{Status: "pending"}
	val, _ := json.Marshal(data)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := config.RDB.Set(ctx, "qrcode:"+ticket, val, 5*time.Minute).Err(); err != nil {
		logger.Log.Error("store qrcode failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	// 二维码内容：前端可将其渲染为二维码，手机端扫描后打开确认页面
	qrcodeURL := fmt.Sprintf("/api/v1/auth/qrcode/confirm?ticket=%s", ticket)
	resp.OK(c, gin.H{
		"ticket":     ticket,
		"qrcode_url": qrcodeURL,
		"expires_in": 300,
	})
}

// GetQrcodeStatus 轮询扫码状态
func GetQrcodeStatus(c *gin.Context) {
	ticket := c.Query("ticket")
	if ticket == "" {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidParam, resp.MsgInvalidParam)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	val, err := config.RDB.GetDel(ctx, "qrcode:"+ticket).Result()
	if err == redis.Nil {
		resp.Fail(c, http.StatusNotFound, resp.CodeQrcodeNotFound, resp.MsgQrcodeNotFound)
		return
	}
	if err != nil {
		logger.Log.Error("get qrcode status failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	var data qrcodeData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	if data.Status == "confirmed" {
		user, err := model.GetUserByID(data.UserID)
		if err != nil {
			resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
			return
		}
		issueAuthResponse(c, user)
		return
	}

	// 未确认状态，需要把数据写回 Redis（因为 GetDel 已经删除了）
	_ = config.RDB.Set(ctx, "qrcode:"+ticket, val, 2*time.Minute).Err()
	resp.OK(c, gin.H{"status": data.Status})
}

// ScanQrcode 手机端扫码（标记为已扫描）
func ScanQrcode(c *gin.Context) {
	type scanReq struct {
		Ticket string `json:"ticket" binding:"required"`
	}
	var req scanReq
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	val, err := config.RDB.Get(ctx, "qrcode:"+req.Ticket).Result()
	if err == redis.Nil {
		resp.Fail(c, http.StatusNotFound, resp.CodeQrcodeExpired, resp.MsgQrcodeExpired)
		return
	}
	if err != nil {
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	var data qrcodeData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	if data.Status != "pending" {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidParam, "qrcode already scanned")
		return
	}

	data.Status = "scanned"
	data.UserID = middleware.CurrentUserID(c)
	newVal, _ := json.Marshal(data)
	ttl, _ := config.RDB.TTL(ctx, "qrcode:"+req.Ticket).Result()
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	_ = config.RDB.Set(ctx, "qrcode:"+req.Ticket, newVal, ttl).Err()

	resp.OK(c, gin.H{"status": "scanned"})
}

// ConfirmQrcode 手机端确认登录
func ConfirmQrcode(c *gin.Context) {
	type confirmReq struct {
		Ticket string `json:"ticket" binding:"required"`
	}
	var req confirmReq
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	val, err := config.RDB.Get(ctx, "qrcode:"+req.Ticket).Result()
	if err == redis.Nil {
		resp.Fail(c, http.StatusNotFound, resp.CodeQrcodeExpired, resp.MsgQrcodeExpired)
		return
	}
	if err != nil {
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	var data qrcodeData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	if data.Status != "scanned" {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidParam, "qrcode not scanned or already confirmed")
		return
	}

	if data.UserID != middleware.CurrentUserID(c) {
		resp.Fail(c, http.StatusForbidden, resp.CodeAuthForbidden, resp.MsgAuthForbidden)
		return
	}

	data.Status = "confirmed"
	newVal, _ := json.Marshal(data)
	ttl, _ := config.RDB.TTL(ctx, "qrcode:"+req.Ticket).Result()
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	_ = config.RDB.Set(ctx, "qrcode:"+req.Ticket, newVal, ttl).Err()

	resp.OK(c, gin.H{"status": "confirmed"})
}
// ========== 邮箱+密码登录（二次验证） ==========

type preAuthData struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Phone  string `json:"phone"`
}

type emailPasswordLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type sendVerifyCodeRequest struct {
	PreAuthToken string `json:"pre_auth_token" binding:"required"`
	Method       string `json:"method" binding:"required,oneof=email sms"`
}

type verifyLoginRequest struct {
	PreAuthToken string `json:"pre_auth_token" binding:"required"`
	Code         string `json:"code" binding:"required,len=6"`
}

// EmailPasswordLogin 第一步：验证邮箱+密码，返回 pre_auth_token
func EmailPasswordLogin(c *gin.Context) {
	var req emailPasswordLoginRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}

	user, err := model.GetUserByEmail(req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			resp.Fail(c, http.StatusUnauthorized, resp.CodeAuthInvalid, resp.MsgLoginFailed)
			return
		}
		logger.Log.Error("email password login query failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	if !user.CheckPassword(req.Password) || user.Status != model.StatusEnabled {
		resp.Fail(c, http.StatusUnauthorized, resp.CodeAuthInvalid, resp.MsgLoginFailed)
		return
	}

	preAuthToken := genTicket()
	data := preAuthData{
		UserID: user.ID,
		Email:  user.Email,
		Phone:  user.Phone,
	}
	val, _ := json.Marshal(data)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := config.RDB.Set(ctx, "pre_auth:"+preAuthToken, val, 5*time.Minute).Err(); err != nil {
		logger.Log.Error("store pre auth failed", zap.Error(err))
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	methods := []string{}
	if user.Email != "" {
		methods = append(methods, "email")
	}
	if user.Phone != "" {
		methods = append(methods, "sms")
	}

	resp.OK(c, gin.H{
		"pre_auth_token": preAuthToken,
		"methods":        methods,
	})
}

// SendVerifyCode 第二步：发送二次验证验证码
func SendVerifyCode(c *gin.Context) {
	var req sendVerifyCodeRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	val, err := config.RDB.Get(ctx, "pre_auth:"+req.PreAuthToken).Result()
	if err == redis.Nil {
		resp.Fail(c, http.StatusBadRequest, resp.CodeAuthInvalid, "pre-auth expired")
		return
	}
	if err != nil {
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	var data preAuthData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	code := genCode()

	if req.Method == "email" {
		if data.Email == "" {
			resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidParam, "email not bound")
			return
		}
		if err := storeCode("verify:email:"+req.PreAuthToken, code, 5*time.Minute); err != nil {
			resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgSendFailed)
			return
		}
		logger.Log.Info("verify code sent", zap.String("email", data.Email), zap.String("code", code))
	} else {
		if data.Phone == "" {
			resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidParam, "phone not bound")
			return
		}
		if err := storeCode("verify:sms:"+req.PreAuthToken, code, 5*time.Minute); err != nil {
			resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgSendFailed)
			return
		}
		logger.Log.Info("verify code sent", zap.String("phone", data.Phone), zap.String("code", code))
	}

	resp.OK(c, gin.H{"expires_in": 300})
}

// VerifyAndLogin 第三步：提交验证码完成登录
func VerifyAndLogin(c *gin.Context) {
	var req verifyLoginRequest
	if err := resp.BindJSON(c, &req); err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 读取 pre_auth
	val, err := config.RDB.Get(ctx, "pre_auth:"+req.PreAuthToken).Result()
	if err == redis.Nil {
		resp.Fail(c, http.StatusBadRequest, resp.CodeAuthInvalid, "pre-auth expired")
		return
	}
	if err != nil {
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	var data preAuthData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	// 验证 code（支持 email 或 sms 任意一种）
	verified := false
	if verifyCode("verify:email:"+req.PreAuthToken, req.Code) {
		verified = true
	} else if verifyCode("verify:sms:"+req.PreAuthToken, req.Code) {
		verified = true
	}

	if !verified {
		resp.Fail(c, http.StatusBadRequest, resp.CodeInvalidCode, resp.MsgInvalidCode)
		return
	}

	// 清理 pre_auth
	_ = config.RDB.Del(ctx, "pre_auth:"+req.PreAuthToken)

	user, err := model.GetUserByID(data.UserID)
	if err != nil {
		resp.Fail(c, http.StatusInternalServerError, resp.CodeInternal, resp.MsgLoginFailed)
		return
	}

	issueAuthResponse(c, user)
}
