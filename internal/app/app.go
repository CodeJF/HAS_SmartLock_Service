package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	cloudhandler "has-smartlock-service/internal/cloud/handler"
	cloudservice "has-smartlock-service/internal/cloud/service"
	devicehandler "has-smartlock-service/internal/device/handler"
	devicerepository "has-smartlock-service/internal/device/repository"
	deviceservice "has-smartlock-service/internal/device/service"
	eventhandler "has-smartlock-service/internal/event/handler"
	eventrepository "has-smartlock-service/internal/event/repository"
	eventservice "has-smartlock-service/internal/event/service"
	homehandler "has-smartlock-service/internal/home/handler"
	homerepository "has-smartlock-service/internal/home/repository"
	homeservice "has-smartlock-service/internal/home/service"
	messagehandler "has-smartlock-service/internal/message/handler"
	messageservice "has-smartlock-service/internal/message/service"
	"has-smartlock-service/internal/pkg/auth"
	"has-smartlock-service/internal/pkg/config"
	"has-smartlock-service/internal/pkg/db"
	"has-smartlock-service/internal/pkg/httpx"
	"has-smartlock-service/internal/pkg/mongox"
	"has-smartlock-service/internal/pkg/protocol"
	"has-smartlock-service/internal/pkg/redisx"
	"has-smartlock-service/internal/pkg/stsclient"
	"has-smartlock-service/internal/realtime"
	eventstore "has-smartlock-service/internal/realtime/eventstore"
	mqttpkg "has-smartlock-service/internal/realtime/mqtt"
	"has-smartlock-service/internal/realtime/shadow"
	wspkg "has-smartlock-service/internal/realtime/ws"
	"has-smartlock-service/internal/user/handler"
	"has-smartlock-service/internal/user/repository"
	"has-smartlock-service/internal/user/service"
)

type App struct {
	config     config.Config
	router     *gin.Engine
	db         *gorm.DB
	eventStore eventstore.Store
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
		config:     cfg,
		router:     router,
		db:         database,
		eventStore: appEventStore,
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

var appEventStore eventstore.Store

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

	redisClient, err := redisx.Open(cfg)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.MongoURI) == "" {
		return errors.New("mongo is required for realtime runtime")
	}
	mongoClient, err := mongox.Open(cfg)
	if err != nil {
		return err
	}

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
	deviceRepo := devicerepository.New(database)
	var shadowStore shadow.Store
	var eventStore eventstore.Store
	if strings.HasPrefix(strings.TrimSpace(cfg.MongoURI), "memory://") {
		shadowStore = shadow.NewMemoryStore()
		eventStore = eventstore.NewMemoryStore()
	} else if db := mongoClient.Database(); db != nil {
		shadowStore = shadow.NewMongoStore(db)
		eventStore = eventstore.NewMongoStore(db)
	}
	if shadowStore == nil || eventStore == nil {
		return errors.New("mongo backing stores not available")
	}
	if err := shadowStore.EnsureIndexes(context.Background()); err != nil {
		return err
	}
	if err := eventStore.EnsureIndexes(context.Background()); err != nil {
		return err
	}
	appEventStore = eventStore
	homeService := homeservice.New(homeRepo, deviceRepo, userRepo, cfg)
	homeHandler := homehandler.New(homeService)
	messageService := messageservice.New(homeRepo, deviceRepo, userRepo)
	messageHandler := messagehandler.New(messageService)
	deviceService := deviceservice.New(deviceRepo, homeRepo, userRepo, redisClient, shadowStore, eventStore, nil, cfg)
	deviceHandler := devicehandler.New(deviceService)
	eventRepo := eventrepository.New(database)
	eventService := eventservice.New(eventRepo, eventStore, deviceRepo, homeRepo, userRepo)
	eventHandler := eventhandler.New(eventService)
	realtimeRuntime := realtime.NewRuntime(deviceRepo, homeRepo, userRepo, redisClient, shadowStore, eventStore, cfg)
	wsHub := wspkg.NewHub(tokenManager, realtimeRuntime.HandleWS)
	realtimeRuntime.SetHub(wsHub)
	mqttClient, err := mqttpkg.Open(cfg, realtimeRuntime)
	if err != nil {
		return err
	}
	realtimeRuntime.SetMQTTClient(mqttClient)
	deviceService.WithMQTTClient(mqttClient)
	deviceService.WithRealtimeNotifier(realtimeRuntime)

	v1 := router.Group("/v1")
	userProtocolMiddleware, err := protocol.UserMiddleware(cfg.AppSecretKey, cfg.SignTimestampSkew)
	if err != nil {
		return err
	}
	deviceModelSecrets, err := protocol.ParseModelSecrets(cfg.DeviceModelSecretsRaw)
	if err != nil {
		return err
	}
	deviceProtocolMiddleware, err := protocol.DeviceMiddleware(deviceModelSecrets, cfg.SignTimestampSkew)
	if err != nil {
		return err
	}
	authMiddleware := auth.Middleware(tokenManager)
	handler.RegisterUserRoutes(v1, userHandler, userProtocolMiddleware, authMiddleware)
	cloudhandler.RegisterCloudRoutes(v1, cloudHandler, userProtocolMiddleware, authMiddleware)
	homehandler.RegisterHomeRoutes(v1, homeHandler, userProtocolMiddleware, authMiddleware)
	messagehandler.RegisterMessageRoutes(v1, messageHandler, userProtocolMiddleware, authMiddleware)
	devicehandler.RegisterDeviceRoutes(v1, deviceHandler, userProtocolMiddleware, deviceProtocolMiddleware, authMiddleware)
	eventhandler.RegisterEventRoutes(v1, eventHandler, userProtocolMiddleware, authMiddleware)
	wsHub.RegisterRoute(router, cfg.WsPath)
	router.GET("/getUrl", func(c *gin.Context) {
		username := c.Query("username")
		exist := false
		if username != "" {
			if _, err := userRepo.FindUserByUsername(username); err == nil {
				exist = true
			}
		}
		httpx.Success(c, gin.H{
			"exist": exist,
			"url": gin.H{
				"api":       cfg.PublicAPIURL,
				"mqtt":      cfg.PublicMqttURL,
				"websocket": cfg.PublicWebSocketURL,
			},
		})
	})
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
