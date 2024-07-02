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
	"github.com/tbxark/go-base-api/pkg/web/auth/tokens"
	"github.com/tbxark/go-base-api/pkg/web/middleware"
	"github.com/tbxark/go-base-api/pkg/wechat"
	"io/fs"
	"net/http"
	"time"
)

type Config struct {
	JWT     string `json:"jwt"`
	Address string `json:"address"`
	Doc     bool   `json:"doc"`
}

type Web struct {
	config *Config
	Engine *gin.Engine
	db     *dao.Dao
	wx     *wechat.Wechat
	cdn    cdn.CDN
	cache  cache.ByteCache
	render *render.Render
	token  *tokens.Generator
	auth   *middleware.JwtAuth
}

func NewWebServer(config *Config, db *dao.Dao, wx *wechat.Wechat, cdn cdn.CDN, cache cache.ByteCache) *Web {
	token := tokens.NewTokenGenerator(config.JWT)
	return &Web{
		config: config,
		Engine: gin.New(),
		db:     db,
		wx:     wx,
		cdn:    cdn,
		cache:  cache,
		render: render.NewRender(cdn, db, false),
		token:  token,
		auth:   middleware.NewJwtAuth(token),
	}
}

func (w *Web) Identifier() string {
	return "dash"
}

func (w *Web) Run() {
	logger := log.ZapLogger().With(field.String("module", "dash"))
	loggerMiddleware := middleware.NewZapLoggerMiddleware(logger)
	recoveryMiddleware := middleware.NewZapRecoveryMiddleware(logger)
	rateLimiter := middleware.NewNewRateLimiterByClientIP(100*time.Millisecond, 10, time.Hour)

	w.Engine.Use(loggerMiddleware, recoveryMiddleware)

	if dash, err := w.dashFs(); err == nil {
		d := w.Engine.Group("/dash", gzip.Gzip(gzip.DefaultCompression))
		d.StaticFS("/", dash)
	}

	api := w.Engine.Group("/")
	auth := api.Group("/", w.auth.NewJwtAuthMiddleware(true))

	if w.config.Doc {
		w.bindDocRoute(api)
	}
	w.bindAdminAuthRoute(api.Group("/", rateLimiter))
	w.bindSystemRoute(auth)

	route := map[string]func(gin.IRouter){
		WebPermissionAdmin: w.bindAdminRoute,
	}
	for page, handler := range route {
		handler(auth.Group("/", w.auth.NewPermissionMiddleware(page)))
	}

	err := w.Engine.Run(w.config.Address)
	if err != nil {
		log.Warnw("dash server run error", field.Error(err))
	}
}

func (w *Web) dashFs() (http.FileSystem, error) {
	sf, err := fs.Sub(assets.DashAssets, assets.DashAssetsPath)
	if err != nil {
		return nil, err
	}
	return http.FS(sf), nil
}
