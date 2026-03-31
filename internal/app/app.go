package app

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"has-smartlock-service/internal/pkg/auth"
	"has-smartlock-service/internal/pkg/config"
	"has-smartlock-service/internal/pkg/db"
	"has-smartlock-service/internal/pkg/httpx"
	"has-smartlock-service/internal/user/handler"
	"has-smartlock-service/internal/user/repository"
	"has-smartlock-service/internal/user/service"
)

type App struct {
	config config.Config
	router *gin.Engine
}

func New() (*App, error) {
	cfg := config.Load()
	database, err := db.Open(cfg)
	if err != nil {
		return nil, err
	}

	return NewWithDependencies(cfg, database)
}

func NewWithDependencies(cfg config.Config, database *gorm.DB) (*App, error) {
	gin.SetMode(ginMode(cfg.AppEnv))

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	registerRoutes(router, cfg, database)

	return &App{
		config: cfg,
		router: router,
	}, nil
}

func (a *App) Run() error {
	return a.router.Run(a.config.HTTPAddr)
}

func (a *App) Router() *gin.Engine {
	return a.router
}

func registerRoutes(router *gin.Engine, cfg config.Config, database *gorm.DB) {
	router.GET("/healthz", func(c *gin.Context) {
		httpx.Success(c, gin.H{
			"status": "ok",
		})
	})

	router.GET("/time", func(c *gin.Context) {
		httpx.Success(c, gin.H{
			"timestamp": time.Now().Unix(),
		})
	})

	userRepo := repository.New(database)
	tokenManager := auth.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	userService := service.New(userRepo, tokenManager, cfg)
	userHandler := handler.New(userService)

	v1 := router.Group("/v1")
	handler.RegisterUserRoutes(v1, userHandler, auth.Middleware(tokenManager))
}

func ginMode(appEnv string) string {
	if appEnv == "production" {
		return gin.ReleaseMode
	}

	if appEnv == "test" {
		return gin.TestMode
	}

	return gin.DebugMode
}
