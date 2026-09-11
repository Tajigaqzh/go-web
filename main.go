package main

import (
	"fmt"

	"go-web/authz"
	"go-web/config"
	"go-web/controller"
	"go-web/logger"
	"go-web/model"
	"go-web/resp"
	"go-web/router"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// @title Go Web API
// @version 1.0
// @description Go Web HTTP 接口文档。
// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description 输入 "Bearer {token}"，例如：Bearer eyJhbGciOiJIUzI1Ni...
func main() {
	cfg := config.LoadConfig()

	if err := logger.Init(&logger.Config{
		Level:    cfg.Log.Level,
		Encoding: cfg.Log.Encoding,
		Dir:      cfg.Log.Dir,
		Rotate:   cfg.Log.Rotate,
		MaxAge:   cfg.Log.MaxAge,
		Console:  cfg.Log.Console,
	}); err != nil {
		panic(err)
	}
	defer logger.Sync()

	resp.Init()
	setGinMode(cfg.Server.Mode)
	config.InitJWT(&cfg.JWT)

	if err := authz.Init(); err != nil {
		logger.Log.Fatal("failed to initialize authz", zap.Error(err))
	}

	if err := model.InitDB(&cfg.Database); err != nil {
		logger.Log.Fatal("failed to initialize database", zap.Error(err))
	}
	defer func() {
		if err := model.CloseDB(); err != nil {
			logger.Log.Error("failed to close database", zap.Error(err))
		}
	}()

	if err := config.InitRedis(&cfg.Redis); err != nil {
		logger.Log.Fatal("failed to initialize redis", zap.Error(err))
	}
	defer func() {
		if err := config.CloseRedis(); err != nil {
			logger.Log.Error("failed to close redis", zap.Error(err))
		}
	}()

	controller.AuthConfig = cfg.Auth

	r := router.SetRouter()

	serverAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	logger.Log.Info("server starting", zap.String("addr", serverAddr))
	if err := r.Run(serverAddr); err != nil {
		logger.Log.Fatal("failed to start server", zap.Error(err))
	}
}

func setGinMode(mode string) {
	switch mode {
	case gin.ReleaseMode, gin.TestMode:
		gin.SetMode(mode)
	default:
		gin.SetMode(gin.DebugMode)
	}
}
