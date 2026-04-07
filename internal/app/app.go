package app

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	cloudhandler "has-smartlock-service/internal/cloud/handler"
	cloudservice "has-smartlock-service/internal/cloud/service"
	homehandler "has-smartlock-service/internal/home/handler"
	homerepository "has-smartlock-service/internal/home/repository"
	homeservice "has-smartlock-service/internal/home/service"
	"has-smartlock-service/internal/pkg/auth"
	"has-smartlock-service/internal/pkg/config"
	"has-smartlock-service/internal/pkg/db"
	"has-smartlock-service/internal/pkg/httpx"
	"has-smartlock-service/internal/pkg/stsclient"
	"has-smartlock-service/internal/user/handler"
	"has-smartlock-service/internal/user/repository"
	"has-smartlock-service/internal/user/service"
)

type App struct {
	config config.Config
	router *gin.Engine
	db     *gorm.DB
}

func New() (*App, error) {
	cfg := config.Load()
	database, err := db.Open(cfg)
	if err != nil {
		return nil, err
	}
	if err := db.RunMigrations(database); err != nil {
		return nil, err
	}

	return NewWithDependencies(cfg, database)
}

func NewWithDependencies(cfg config.Config, database *gorm.DB) (*App, error) {
	gin.SetMode(ginMode(cfg.AppEnv))

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	if err := registerRoutes(router, cfg, database); err != nil {
		return nil, err
	}

	return &App{
		config: cfg,
		router: router,
		db:     database,
	}, nil
}

func (a *App) Run() error {
	return a.router.Run(a.config.HTTPAddr)
}

func (a *App) Router() *gin.Engine {
	return a.router
}

func (a *App) DB() *gorm.DB {
	return a.db
}

func registerRoutes(router *gin.Engine, cfg config.Config, database *gorm.DB) error {
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
	stsTokenClient, err := stsclient.New(cfg)
	if err != nil {
		return err
	}
	userService := service.New(userRepo, tokenManager, cfg)
	userHandler := handler.New(userService)
	cloudService := cloudservice.New(userRepo, stsTokenClient)
	cloudHandler := cloudhandler.New(cloudService)
	homeRepo := homerepository.New(database)
	homeService := homeservice.New(homeRepo, userRepo, cfg)
	homeHandler := homehandler.New(homeService)

	v1 := router.Group("/v1")
	authMiddleware := auth.Middleware(tokenManager)
	handler.RegisterUserRoutes(v1, userHandler, authMiddleware)
	cloudhandler.RegisterCloudRoutes(v1, cloudHandler, authMiddleware)
	homehandler.RegisterHomeRoutes(v1, homeHandler, authMiddleware)
	return nil
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
