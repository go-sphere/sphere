package dash

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/internal/pkg/dao"
	"github.com/tbxark/go-base-api/internal/pkg/render"
	"github.com/tbxark/go-base-api/pkg/cache"
	"github.com/tbxark/go-base-api/pkg/cdn"
	"github.com/tbxark/go-base-api/pkg/log"
	"github.com/tbxark/go-base-api/pkg/log/logfields"
	"github.com/tbxark/go-base-api/pkg/web"
	"github.com/tbxark/go-base-api/pkg/web/auth/jwtauth"
	"github.com/tbxark/go-base-api/pkg/web/middleware/auth"
	"github.com/tbxark/go-base-api/pkg/web/middleware/logger"
	"github.com/tbxark/go-base-api/pkg/web/middleware/ratelimiter"
	"github.com/tbxark/go-base-api/pkg/web/webmodels"
	"github.com/tbxark/go-base-api/pkg/wechat"
	"time"
)

type Config struct {
	JWT        string `json:"jwt"`
	Address    string `json:"address"`
	Doc        bool   `json:"doc"`
	DashCors   string `json:"dash_cors"`
	DashStatic string `json:"dash_static"`
}

type Web struct {
	config *Config
	Engine *gin.Engine
	db     *dao.Dao
	wx     *wechat.Wechat
	cdn    cdn.CDN
	cache  cache.ByteCache
	render *render.Render
	token  *jwtauth.JwtAuth
	auth   *auth.Auth
}

func NewWebServer(config *Config, db *dao.Dao, wx *wechat.Wechat, cdn cdn.CDN, cache cache.ByteCache) *Web {
	token := jwtauth.NewJwtAuth(config.JWT)
	return &Web{
		config: config,
		Engine: gin.New(),
		db:     db,
		wx:     wx,
		cdn:    cdn,
		cache:  cache,
		render: render.NewRender(cdn, db, false),
		token:  token,
		auth:   auth.NewAuth(jwtauth.AuthorizationPrefixBearer, token),
	}
}

type MessageResponse = web.DataResponse[webmodels.MessageResponse]

func (w *Web) Identifier() string {
	return "dash"
}

func (w *Web) Run() error {
	zapLogger := log.ZapLogger().With(logfields.String("module", "dash"))
	loggerMiddleware := logger.NewZapLoggerMiddleware(zapLogger)
	recoveryMiddleware := logger.NewZapRecoveryMiddleware(zapLogger)
	rateLimiter := ratelimiter.NewNewRateLimiterByClientIP(100*time.Millisecond, 10, time.Hour)

	w.Engine.Use(loggerMiddleware, recoveryMiddleware)

	// 1. 使用go直接反代
	// ignore embed: web.Fs(w.config.DashStatic, nil, "")
	// 2. 使用embed集成
	// with embed: web.Fs("", &dash.Assets, dash.AssetsPath)
	if dashFs, err := web.Fs(w.config.DashStatic, nil, ""); err == nil && dashFs != nil {
		d := w.Engine.Group("/dash", gzip.Gzip(gzip.DefaultCompression))
		d.StaticFS("/", dashFs)
	}
	// 3. 使用其他服务反代但是允许其跨域访问
	// 其中w.config.DashCors是一个配置项，用于配置允许跨域访问的域名,例如：https://dash.example.com
	if w.config.DashCors != "" {
		w.Engine.Use(cors.New(cors.Config{
			AllowOrigins:     []string{w.config.DashCors},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}))
	}

	api := w.Engine.Group("/")
	authRoute := api.Group("/", w.auth.NewAuthMiddleware(true))

	if w.config.Doc {
		w.bindDocRoute(api)
	}
	w.bindAdminAuthRoute(api.Group("/", rateLimiter))
	w.bindSystemRoute(authRoute)
	w.bindAdminRoute(authRoute)

	return w.Engine.Run(w.config.Address)
}
