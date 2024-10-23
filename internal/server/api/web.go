package api

import (
	"github.com/gin-gonic/gin"
	apiv1 "github.com/tbxark/sphere/api/api/v1"
	sharedv1 "github.com/tbxark/sphere/api/shared/v1"
	"github.com/tbxark/sphere/internal/service/api"
	"github.com/tbxark/sphere/internal/service/shared"
	"github.com/tbxark/sphere/pkg/log"
	"github.com/tbxark/sphere/pkg/log/logfields"
	"github.com/tbxark/sphere/pkg/server/auth/jwtauth"
	"github.com/tbxark/sphere/pkg/server/middleware/auth"
	"github.com/tbxark/sphere/pkg/server/middleware/logger"
	"github.com/tbxark/sphere/pkg/server/route/cors"
)

type Web struct {
	config  *Config
	engine  *gin.Engine
	service *api.Service
}

func NewWebServer(conf *Config, service *api.Service) *Web {
	return &Web{
		config:  conf,
		engine:  gin.New(),
		service: service,
	}
}

func (w *Web) Identifier() string {
	return "api"
}

func (w *Web) Run() error {

	authorizer := jwtauth.NewJwtAuth[int64](w.config.JWT)
	authControl := auth.NewAuth[int64](jwtauth.AuthorizationPrefixBearer, authorizer)

	zapLogger := log.ZapLogger().With(logfields.String("module", "api"))
	loggerMiddleware := logger.NewZapLoggerMiddleware(zapLogger)
	recoveryMiddleware := logger.NewZapRecoveryMiddleware(zapLogger)
	authMiddleware := authControl.NewAuthMiddleware(false)
	//rateLimiter := middleware.NewNewRateLimiterByClientIP(100*time.Millisecond, 10, time.Hour)

	w.engine.Use(loggerMiddleware, recoveryMiddleware)

	if len(w.config.HTTP.Cors) > 0 {
		cors.Setup(w.engine, w.config.HTTP.Cors)
	}

	w.service.Init(authControl, authorizer)

	route := w.engine.Group("/", authMiddleware)

	sharedSrc := shared.NewService(authControl, w.service.Storage, "user")

	sharedv1.RegisterStorageServiceHTTPServer(route, sharedSrc)
	apiv1.RegisterAuthServiceHTTPServer(route, w.service)
	apiv1.RegisterSystemServiceHTTPServer(route, w.service)
	apiv1.RegisterUserServiceHTTPServer(route, w.service)

	return w.engine.Run(w.config.HTTP.Address)
}
