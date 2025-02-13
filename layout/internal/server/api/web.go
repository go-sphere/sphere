package api

import (
	"context"
	apiv1 "github.com/TBXark/sphere/layout/api/api/v1"
	"github.com/TBXark/sphere/layout/api/shared/v1"
	"github.com/TBXark/sphere/layout/internal/service/api"
	"github.com/TBXark/sphere/layout/internal/service/shared"
	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/TBXark/sphere/server/auth/jwtauth"
	"github.com/TBXark/sphere/server/ginx"
	"github.com/TBXark/sphere/server/middleware/auth"
	"github.com/TBXark/sphere/server/middleware/logger"
	"github.com/TBXark/sphere/server/route/cors"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

type Web struct {
	config  *Config
	server  *http.Server
	service *api.Service
}

func NewWebServer(conf *Config, service *api.Service) *Web {
	return &Web{
		config:  conf,
		service: service,
	}
}

func (w *Web) Identifier() string {
	return "api"
}

func (w *Web) Start(ctx context.Context) error {
	jwtAuthorizer := jwtauth.NewJwtAuth[authorizer.RBACClaims[int64]](w.config.JWT)

	zapLogger := log.ZapLogger().With(logfields.String("module", "api"))
	loggerMiddleware := logger.NewZapLoggerMiddleware(zapLogger)
	recoveryMiddleware := logger.NewZapRecoveryMiddleware(zapLogger)
	authMiddleware := auth.NewAuthMiddleware(jwtauth.AuthorizationPrefixBearer, jwtAuthorizer, false)
	//rateLimiter := middleware.NewNewRateLimiterByClientIP(100*time.Millisecond, 10, time.Hour)

	engine := gin.New()
	engine.Use(loggerMiddleware, recoveryMiddleware)

	if len(w.config.HTTP.Cors) > 0 {
		cors.Setup(engine, w.config.HTTP.Cors)
	}

	w.service.Init(jwtAuthorizer)

	route := engine.Group("/", authMiddleware)

	sharedSrc := shared.NewService(w.service.Storage, "user")

	sharedv1.RegisterStorageServiceHTTPServer(route, sharedSrc)
	apiv1.RegisterAuthServiceHTTPServer(route, w.service)
	apiv1.RegisterSystemServiceHTTPServer(route, w.service)
	apiv1.RegisterUserServiceHTTPServer(route, w.service)

	w.server = &http.Server{
		Addr:    w.config.HTTP.Address,
		Handler: engine.Handler(),
	}
	return ginx.Start(ctx, w.server, 30*time.Second)
}

func (w *Web) Stop(ctx context.Context) error {
	return ginx.Close(ctx, w.server)
}
