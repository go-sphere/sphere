package api

import (
	"context"
	"github.com/gin-gonic/gin"
	apiv1 "github.com/tbxark/sphere/api/api/v1"
	sharedv1 "github.com/tbxark/sphere/api/shared/v1"
	"github.com/tbxark/sphere/internal/service/api"
	"github.com/tbxark/sphere/internal/service/shared"
	"github.com/tbxark/sphere/pkg/log"
	"github.com/tbxark/sphere/pkg/log/logfields"
	"github.com/tbxark/sphere/pkg/server/auth/authorizer"
	"github.com/tbxark/sphere/pkg/server/auth/jwtauth"
	"github.com/tbxark/sphere/pkg/server/middleware/auth"
	"github.com/tbxark/sphere/pkg/server/middleware/logger"
	"github.com/tbxark/sphere/pkg/server/route/cors"
	"net/http"
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

func (w *Web) Run() error {
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
	return w.server.ListenAndServe()
}

func (w *Web) Close(ctx context.Context) error {
	if w.server != nil {
		err := w.server.Close()
		if err != nil {
			return err
		}
		w.server = nil
	}
	return nil
}
