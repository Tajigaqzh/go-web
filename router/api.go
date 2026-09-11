package router

import (
	"time"

	"go-web/authz"
	"go-web/controller"
	"go-web/middleware"

	"github.com/gin-gonic/gin"
)

// SetApiRouter 所有业务接口统一前缀: /api/v1
// 需登录的接口: Header 带 Authorization: Bearer <access_token>
// RequirePermission(...) 不是 URL 的一部分，只是登录后的 Casbin 权限校验。
func SetApiRouter(r *gin.Engine) {
	api := r.Group("/api/v1")

	// ---- 公开（无需 Token）----
	// POST /api/v1/register
	api.POST(
		"/register",
		middleware.RateLimitGlobal("register", 100, time.Hour),
		middleware.RateLimitByIP("register", 5, time.Hour),
		controller.Register,
	)
	// POST /api/v1/login
	api.POST(
		"/login",
		middleware.RateLimitGlobal("login", 1000, time.Minute),
		middleware.RateLimitByIP("login", 10, time.Minute),
		controller.Login,
	)
	// POST /api/v1/refresh
	api.POST(
		"/refresh",
		middleware.RateLimitGlobal("refresh", 300, time.Minute),
		middleware.RateLimitByIP("refresh", 10, time.Minute),
		controller.Refresh,
	)

	// ---- 加密公钥（公开）----
	api.GET("/auth/public-key", controller.GetPublicKey)

	// ---- 验证码与第三方登录（公开）----
	api.POST("/auth/sms/send", middleware.RateLimitByIP("sms_send", 3, time.Minute), controller.SendSmsCode)
	api.POST("/auth/sms/login", middleware.RateLimitByIP("sms_login", 10, time.Minute), controller.SmsLogin)
	api.POST("/auth/sms/register", middleware.RateLimitByIP("sms_register", 5, time.Minute), controller.SmsRegister)
	api.POST("/auth/email/send", middleware.RateLimitByIP("email_send", 3, time.Minute), controller.SendEmailCode)
	api.POST("/auth/email/login", middleware.RateLimitByIP("email_login", 10, time.Minute), controller.EmailLogin)
	api.POST("/auth/email/register", middleware.RateLimitByIP("email_register", 5, time.Minute), controller.EmailRegister)
	api.POST("/auth/wechat", middleware.RateLimitByIP("wechat_login", 20, time.Minute), controller.WechatLogin)
	api.POST("/auth/qrcode", controller.GetQrcode)
	api.GET("/auth/qrcode/status", controller.GetQrcodeStatus)

	// ---- 邮箱+密码登录（二次验证）----
	api.POST("/auth/login/email-password", middleware.RateLimitByIP("email_password_login", 10, time.Minute), controller.EmailPasswordLogin)
	api.POST("/auth/login/send-verify-code", middleware.RateLimitByIP("send_verify_code", 3, time.Minute), controller.SendVerifyCode)
	api.POST("/auth/login/verify", middleware.RateLimitByIP("verify_login", 10, time.Minute), controller.VerifyAndLogin)

	// ---- OAuth2 跳转登录（公开）----
	api.GET("/oauth/github/authorize", controller.GitHubAuthorize)
	api.GET("/oauth/github/callback", controller.GitHubCallback)
	api.GET("/oauth/google/authorize", controller.GoogleAuthorize)
	api.GET("/oauth/google/callback", controller.GoogleCallback)
	api.GET("/oauth/wechat/authorize", controller.WechatAuthorize)
	api.GET("/oauth/wechat/callback", controller.WechatCallback)

	// ---- 以下全部需要 Auth() ----
	api.Use(middleware.Auth())
	{
		// GET  /api/v1/me
		api.GET("/me", controller.Me)
		// GET  /api/v1/notifications
		api.GET("/notifications", controller.ListNotifications)
		// POST /api/v1/notifications/:id/read
		api.POST("/notifications/:id/read", controller.ReadNotification)

		// 扫码登录（手机端已登录后操作）
		api.POST("/auth/qrcode/scan", controller.ScanQrcode)
		api.POST("/auth/qrcode/confirm", controller.ConfirmQrcode)

		users := api.Group("/users")
		{
			// GET  /api/v1/users                    perm: user:read
			users.GET("", middleware.RequirePermission(authz.UserRead), controller.GetUsers)
			// POST /api/v1/users                    perm: user:write
			users.POST("", middleware.RequirePermission(authz.UserWrite), controller.CreateUser)
			// POST /api/v1/users/:id/ban            perm: user:write
			users.POST("/:id/ban", middleware.RequirePermission(authz.UserWrite), controller.BanUser)
			// POST /api/v1/users/:id/freeze         perm: user:write
			users.POST("/:id/freeze", middleware.RequirePermission(authz.UserWrite), controller.FreezeUser)
			// POST /api/v1/users/:id/delete-account perm: user:write
			users.POST("/:id/delete-account", middleware.RequirePermission(authz.UserWrite), controller.DeleteUser)
			// POST /api/v1/users/:id/restore        perm: user:write
			users.POST("/:id/restore", middleware.RequirePermission(authz.UserWrite), controller.RestoreUser)
		}

		canvases := api.Group("/canvases")
		{
			// GET    /api/v1/canvases               perm: canvas:read
			canvases.GET("", middleware.RequirePermission(authz.CanvasRead), controller.ListCanvases)
			// POST   /api/v1/canvases               perm: canvas:write
			canvases.POST("", middleware.RequirePermission(authz.CanvasWrite), controller.CreateCanvas)
			// GET    /api/v1/canvases/:id           perm: canvas:read
			canvases.GET("/:id", middleware.RequirePermission(authz.CanvasRead), controller.GetCanvas)
			// POST   /api/v1/canvases/updateById/:id    perm: canvas:write
			canvases.POST("/updateById/:id", middleware.RequirePermission(authz.CanvasWrite), controller.UpdateCanvas)
			// POST   /api/v1/canvases/deleteById/:id    perm: canvas:write
			canvases.POST("/deleteById/:id", middleware.RequirePermission(authz.CanvasWrite), controller.DeleteCanvas)
			// POST   /api/v1/canvases/:id/submit    perm: canvas:write
			canvases.POST("/:id/submit", middleware.RequirePermission(authz.CanvasWrite), controller.SubmitCanvas)
		}

		audits := api.Group("/audits/canvases")
		audits.Use(middleware.RequirePermission(authz.CanvasAudit))
		{
			// GET  /api/v1/audits/canvases              perm: canvas:audit
			audits.GET("", controller.ListPendingCanvases)
			// POST /api/v1/audits/canvases/:id/approve  perm: canvas:audit
			audits.POST("/:id/approve", controller.ApproveCanvas)
			// POST /api/v1/audits/canvases/:id/reject   perm: canvas:audit
			audits.POST("/:id/reject", controller.RejectCanvas)
		}

		vip := api.Group("/publisher/vip-plans")
		{
			// GET  /api/v1/publisher/vip-plans
			vip.GET("", controller.ListMyVipPlans)
			// POST /api/v1/publisher/vip-plans
			vip.POST("", controller.CreateVipPlan)
		}
		// GET  /api/v1/publishers/:id/vip-plans
		api.GET("/publishers/:id/vip-plans", controller.ListPublisherVipPlans)
		// POST /api/v1/publishers/:id/subscribe
		api.POST("/publishers/:id/subscribe", controller.SubscribePublisherVip)

		materials := api.Group("/materials")
		{
			// GET  /api/v1/materials                perm: material:read
			materials.GET("", middleware.RequirePermission(authz.MaterialRead), controller.ListMaterials)
			// POST /api/v1/materials                perm: material:write
			materials.POST("", middleware.RequirePermission(authz.MaterialWrite), controller.CreateMaterial)
		}
	}
}
