package dash

import (
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/assets"
	"github.com/tbxark/go-base-api/internal/pkg/dao"
	"github.com/tbxark/go-base-api/internal/pkg/render"
	"github.com/tbxark/go-base-api/pkg/cache"
	"github.com/tbxark/go-base-api/pkg/cdn"
	"github.com/tbxark/go-base-api/pkg/log"
	"github.com/tbxark/go-base-api/pkg/log/field"
	"github.com/tbxark/go-base-api/pkg/web"
	"github.com/tbxark/go-base-api/pkg/web/auth/jwt_tokens"
	"github.com/tbxark/go-base-api/pkg/web/middleware/auth"
	"github.com/tbxark/go-base-api/pkg/web/middleware/logger"
	"github.com/tbxark/go-base-api/pkg/web/middleware/ratelimiter"
	"github.com/tbxark/go-base-api/pkg/wechat"
	"time"
)

type Config struct {
	JWT           string `json:"jwt"`
	Address       string `json:"address"`
	Doc           bool   `json:"doc"`
	DashLocalPath string `json:"dash_local_path"`
}

type Web struct {
	config *Config
	Engine *gin.Engine
	db     *dao.Dao
	wx     *wechat.Wechat
	cdn    cdn.CDN
	cache  cache.ByteCache
	render *render.Render
	token  *jwt_tokens.JwtAuth
	auth   *auth.Auth
}

func NewWebServer(config *Config, db *dao.Dao, wx *wechat.Wechat, cdn cdn.CDN, cache cache.ByteCache) *Web {
	token := jwt_tokens.NewJwtAuth(config.JWT)
	return &Web{
		config: config,
		Engine: gin.New(),
		db:     db,
		wx:     wx,
		cdn:    cdn,
		cache:  cache,
		render: render.NewRender(cdn, db, false),
		token:  token,
		auth:   auth.NewJwtAuth(jwt_tokens.AuthorizationPrefixBearer, token),
	}
}

func (w *Web) Identifier() string {
	return "dash"
}

func (w *Web) Run() {
	zapLogger := log.ZapLogger().With(field.String("module", "dash"))
	loggerMiddleware := logger.NewZapLoggerMiddleware(zapLogger)
	recoveryMiddleware := logger.NewZapRecoveryMiddleware(zapLogger)
	rateLimiter := ratelimiter.NewNewRateLimiterByClientIP(100*time.Millisecond, 10, time.Hour)

	w.Engine.Use(loggerMiddleware, recoveryMiddleware)

	if dash, err := web.Fs(w.config.DashLocalPath, assets.DashAssets, assets.DashAssetsPath); err == nil {
		d := w.Engine.Group("/dash", gzip.Gzip(gzip.DefaultCompression))
		d.StaticFS("/", dash)
	}

	api := w.Engine.Group("/")
	authGroup := api.Group("/", w.auth.NewAuthMiddleware(true))

	if w.config.Doc {
		w.bindDocRoute(api)
	}
	w.bindAdminAuthRoute(api.Group("/", rateLimiter))
	w.bindSystemRoute(authGroup)

	route := map[string]func(gin.IRouter){
		WebPermissionAdmin: w.bindAdminRoute,
	}
	for page, handler := range route {
		handler(authGroup.Group("/", w.auth.NewPermissionMiddleware(page)))
	}

	err := w.Engine.Run(w.config.Address)
	if err != nil {
		log.Warnw("dash server run error", field.Error(err))
	}
}
