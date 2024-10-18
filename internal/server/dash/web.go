package dash

import (
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	dashv1 "github.com/tbxark/sphere/api/dash/v1"
	sharedv1 "github.com/tbxark/sphere/api/shared/v1"
	"github.com/tbxark/sphere/internal/service/dash"
	"github.com/tbxark/sphere/internal/service/shared"
	"github.com/tbxark/sphere/pkg/log"
	"github.com/tbxark/sphere/pkg/log/logfields"
	"github.com/tbxark/sphere/pkg/web/auth/jwtauth"
	"github.com/tbxark/sphere/pkg/web/ginx"
	"github.com/tbxark/sphere/pkg/web/middleware/auth"
	"github.com/tbxark/sphere/pkg/web/middleware/logger"
	"github.com/tbxark/sphere/pkg/web/middleware/ratelimiter"
	"github.com/tbxark/sphere/pkg/web/route/cors"
	"github.com/tbxark/sphere/pkg/web/route/pprof"
	"time"
)

type Web struct {
	config  *Config
	engine  *gin.Engine
	service *dash.Service
}

func NewWebServer(config *Config, service *dash.Service) *Web {
	return &Web{
		config:  config,
		engine:  gin.New(),
		service: service,
	}
}

func (w *Web) Identifier() string {
	return "dash"
}

const (
	WebPermissionAll   = "all"
	WebPermissionAdmin = "admin"
)

func (w *Web) Run() error {
	authorizer := jwtauth.NewJwtAuth(w.config.JWT)
	authControl := auth.NewAuth(jwtauth.AuthorizationPrefixBearer, authorizer)

	zapLogger := log.ZapLogger().With(logfields.String("module", "dash"))
	loggerMiddleware := logger.NewZapLoggerMiddleware(zapLogger)
	recoveryMiddleware := logger.NewZapRecoveryMiddleware(zapLogger)
	authMiddleware := authControl.NewAuthMiddleware(true)
	rateLimiter := ratelimiter.NewNewRateLimiterByClientIP(100*time.Millisecond, 10, time.Hour)

	w.engine.Use(loggerMiddleware, recoveryMiddleware)

	// 1. 使用go直接反代
	// ignore embed: web.Fs(w.config.DashStatic, nil, "")
	// 2. 使用embed集成
	// with embed: web.Fs("", &dash.Assets, dash.AssetsPath)
	if dashFs, err := ginx.Fs(w.config.HTTP.Static, nil, ""); err == nil && dashFs != nil {
		d := w.engine.Group("/dash", gzip.Gzip(gzip.DefaultCompression))
		d.StaticFS("/", dashFs)
	}
	// 3. 使用其他服务反代但是允许其跨域访问
	// 其中w.config.DashCors是一个配置项，用于配置允许跨域访问的域名,例如：https://dash.example.com
	if len(w.config.HTTP.Cors) > 0 {
		cors.Setup(w.engine, w.config.HTTP.Cors)
	}

	api := w.engine.Group("/")
	needAuthRoute := api.Group("/", authMiddleware)

	w.service.Init(authControl, authorizer)

	if w.config.HTTP.PProf {
		pprof.SetupPProf(api)
	}
	initDefaultRolesACL(w.service.ACL)

	sharedSrc := shared.NewService(authControl, w.service.Storage, "dash")
	sharedv1.RegisterStorageServiceHTTPServer(needAuthRoute, sharedSrc)
	sharedv1.RegisterTestServiceHTTPServer(api, sharedSrc)

	authRoute := api.Group("/", rateLimiter)
	dashv1.RegisterAuthServiceHTTPServer(authRoute, w.service)

	adminRoute := needAuthRoute.Group("/", w.NewPermissionMiddleware(WebPermissionAdmin))
	dashv1.RegisterAdminServiceHTTPServer(adminRoute, w.service)

	systemRoute := needAuthRoute.Group("/")
	dashv1.RegisterSystemServiceHTTPServer(systemRoute, w.service)

	return w.engine.Run(w.config.HTTP.Address)
}

func (w *Web) NewPermissionMiddleware(resource string) gin.HandlerFunc {
	return w.service.Auth.NewPermissionMiddleware(resource, w.service.ACL)
}

func initDefaultRolesACL(acl *auth.ACL) {
	roles := []string{
		WebPermissionAdmin,
	}
	for _, r := range roles {
		acl.Allow(WebPermissionAll, r)
		acl.Allow(r, r)
	}
}
