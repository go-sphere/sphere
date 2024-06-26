package dash

import (
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/assets"
	"github.com/tbxark/go-base-api/internal/biz/render"
	"github.com/tbxark/go-base-api/internal/pkg/dao"
	"github.com/tbxark/go-base-api/pkg/cache"
	"github.com/tbxark/go-base-api/pkg/log"
	"github.com/tbxark/go-base-api/pkg/log/field"
	"github.com/tbxark/go-base-api/pkg/qniu"
	"github.com/tbxark/go-base-api/pkg/web/middleware"
	"github.com/tbxark/go-base-api/pkg/wechat"
	"io/fs"
	"net/http"
	"time"
)

type Config struct {
	JWT     string `json:"jwt"`
	Address string `json:"address"`
}

type Web struct {
	config *Config
	gin    *gin.Engine
	db     *dao.Database
	wx     *wechat.Wechat
	cdn    *qniu.CDN
	cache  cache.ByteCache
	render *render.Render
	auth   *middleware.JwtAuth
}

func NewWebServer(config *Config, db *dao.Database, wx *wechat.Wechat, cdn *qniu.CDN, cache cache.ByteCache) *Web {
	return &Web{
		config: config,
		gin:    gin.New(),
		db:     db,
		wx:     wx,
		cdn:    cdn,
		cache:  cache,
		render: render.NewRender(cdn, db, false),
		auth:   middleware.NewJwtAuth(config.JWT),
	}
}

func (w *Web) Identifier() string {
	return "dash"
}

func (w *Web) Run() {
	logger := log.With(map[string]interface{}{
		"module": "dash",
	})
	loggerMiddleware := middleware.NewZapLoggerMiddleware(logger)
	recoveryMiddleware := middleware.NewZapRecoveryMiddleware(logger)
	rateLimiter := middleware.NewNewRateLimiterByClientIP(100*time.Millisecond, 10, time.Hour)

	w.gin.Use(loggerMiddleware, recoveryMiddleware)

	if dash, err := w.dashFs(); err == nil {
		d := w.gin.Group("/dash", gzip.Gzip(gzip.DefaultCompression))
		d.StaticFS("/", dash)
	}

	api := w.gin.Group("/")

	w.bindAdminAuthRoute(api.Group("/", rateLimiter))

	auth := api.Group("/", w.auth.NewJwtAuthMiddleware(true))

	w.bindSystemRoute(auth)
	route := map[string]func(gin.IRouter){
		WebPermissionAdmin: w.bindAdminRoute,
	}
	for page, handler := range route {
		handler(auth.Group("/", w.auth.NewPermissionMiddleware(page)))
	}

	err := w.gin.Run(w.config.Address)
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
