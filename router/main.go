package router

import (
	_ "go-web/docs"
	"go-web/logger"
	"go-web/middleware"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetRouter() *gin.Engine {
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(middleware.CORS())
	r.Use(gin.Recovery())
	r.Use(logger.GinLogger())
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	SetApiRouter(r)
	return r
}
