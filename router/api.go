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

	// ---- 以下全部需要 Auth() ----
	api.Use(middleware.Auth())
	{
		// GET  /api/v1/me
		api.GET("/me", controller.Me)
		// GET  /api/v1/notifications
		api.GET("/notifications", controller.ListNotifications)
		// POST /api/v1/notifications/:id/read
		api.POST("/notifications/:id/read", controller.ReadNotification)

		users := api.Group("/users")
		{
			// GET  /api/v1/users                    perm: user:read
			users.GET("", middleware.RequirePermission(authz.UserRead), controller.GetUsers)
			// POST /api/v1/users                    perm: user:write
			users.POST("", middleware.RequirePermission(authz.UserWrite), controller.CreateUser)
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
		api.POST("/publishers/:id/subscribe", controller.SubscribePublisherVIP)

		materials := api.Group("/materials")
		{
			// GET  /api/v1/materials                perm: material:read
			materials.GET("", middleware.RequirePermission(authz.MaterialRead), controller.ListMaterials)
			// POST /api/v1/materials                perm: material:write
			materials.POST("", middleware.RequirePermission(authz.MaterialWrite), controller.CreateMaterial)
		}
	}
}
