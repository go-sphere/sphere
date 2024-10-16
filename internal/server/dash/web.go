package dash

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/tbxark/sphere/internal/pkg/dao"
	"github.com/tbxark/sphere/pkg/cache"
	"github.com/tbxark/sphere/pkg/log"
	"github.com/tbxark/sphere/pkg/log/logfields"
	"github.com/tbxark/sphere/pkg/storage"
	"github.com/tbxark/sphere/pkg/utils/render"
	"github.com/tbxark/sphere/pkg/web"
	"github.com/tbxark/sphere/pkg/web/auth/jwtauth"
	"github.com/tbxark/sphere/pkg/web/middleware/auth"
	"github.com/tbxark/sphere/pkg/web/middleware/logger"
	"github.com/tbxark/sphere/pkg/web/middleware/ratelimiter"
	"github.com/tbxark/sphere/pkg/wechat"
	"time"
)

// @title Dash
// @version 1.0.0
// @description Dash docs
// @accept json
// @produce json

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @description JWT token

type Config struct {
	JWT        string `json:"jwt"`
	Address    string `json:"address"`
	Doc        bool   `json:"doc"`
	DashCors   string `json:"dash_cors"`
	DashStatic string `json:"dash_static"`
}

type Web struct {
	config  *Config
	engine  *gin.Engine
	DB      *dao.Dao
	Storage storage.Storage
	Cache   cache.ByteCache
	WeChat  *wechat.Wechat
	Render  *render.Render
	JwtAuth *jwtauth.JwtAuth
	Auth    *auth.Auth
	ACL     *auth.ACL
}

func NewWebServer(config *Config, db *dao.Dao, wx *wechat.Wechat, store storage.Storage, cache cache.ByteCache) *Web {
	token := jwtauth.NewJwtAuth(config.JWT)
	return &Web{
		config:  config,
		engine:  gin.New(),
		DB:      db,
		Storage: store,
		Cache:   cache,
		WeChat:  wx,
		Render:  render.NewRender(store, db, false),
		JwtAuth: token,
		Auth:    auth.NewAuth(jwtauth.AuthorizationPrefixBearer, token),
		ACL:     NewDefaultRolesACL(),
	}
}

func (w *Web) Identifier() string {
	return "dash"
}

func (w *Web) Run() error {
	zapLogger := log.ZapLogger().With(logfields.String("module", "dash"))
	loggerMiddleware := logger.NewZapLoggerMiddleware(zapLogger)
	recoveryMiddleware := logger.NewZapRecoveryMiddleware(zapLogger)
	rateLimiter := ratelimiter.NewNewRateLimiterByClientIP(100*time.Millisecond, 10, time.Hour)

	w.engine.Use(loggerMiddleware, recoveryMiddleware)

	// 1. 使用go直接反代
	// ignore embed: web.Fs(w.config.DashStatic, nil, "")
	// 2. 使用embed集成
	// with embed: web.Fs("", &dash.Assets, dash.AssetsPath)
	if dashFs, err := web.Fs(w.config.DashStatic, nil, ""); err == nil && dashFs != nil {
		d := w.engine.Group("/dash", gzip.Gzip(gzip.DefaultCompression))
		d.StaticFS("/", dashFs)
	}
	// 3. 使用其他服务反代但是允许其跨域访问
	// 其中w.config.DashCors是一个配置项，用于配置允许跨域访问的域名,例如：https://dash.example.com
	if w.config.DashCors != "" {
		w.engine.Use(cors.New(cors.Config{
			AllowOrigins:     []string{w.config.DashCors},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}))
	}

	api := w.engine.Group("/")
	authRoute := api.Group("/", w.Auth.NewAuthMiddleware(true))

	if w.config.Doc {
		w.bindDocRoute(api)
	}
	w.bindAuthRoute(api.Group("/", rateLimiter))
	w.bindSystemRoute(authRoute)
	w.bindAdminRoute(authRoute)

	return w.engine.Run(w.config.Address)
}

func NewDefaultRolesACL() *auth.ACL {
	acl := auth.NewACL()
	roles := []string{
		WebPermissionAdmin,
	}
	for _, r := range roles {
		acl.Allow(WebPermissionAll, r)
		acl.Allow(r, r)
	}
	return acl
}

func (w *Web) NewPermissionMiddleware(resource string) gin.HandlerFunc {
	return w.Auth.NewPermissionMiddleware(resource, w.ACL)
}
