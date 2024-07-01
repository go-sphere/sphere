package api

import (
	"github.com/gin-gonic/gin"
	"github.com/tbxark/go-base-api/internal/pkg/dao"
	"github.com/tbxark/go-base-api/internal/pkg/render"
	"github.com/tbxark/go-base-api/pkg/cache"
	"github.com/tbxark/go-base-api/pkg/log"
	"github.com/tbxark/go-base-api/pkg/log/field"
	"github.com/tbxark/go-base-api/pkg/qniu"
	"github.com/tbxark/go-base-api/pkg/web/auth/tokens"
	"github.com/tbxark/go-base-api/pkg/web/middleware"
	"github.com/tbxark/go-base-api/pkg/wechat"
	"golang.org/x/sync/singleflight"
	"net/http"
	"strconv"
	"strings"
)

type Config struct {
	JWT     string `json:"jwt"`
	Address string `json:"address"`
}

type Web struct {
	config *Config
	gin    *gin.Engine
	sf     singleflight.Group
	db     *dao.Database
	wx     *wechat.Wechat
	cdn    *qniu.CDN
	cache  cache.ByteCache
	render *render.Render
	token  *tokens.Generator
	auth   *middleware.JwtAuth
}

func NewWebServer(config *Config, db *dao.Database, wx *wechat.Wechat, cdn *qniu.CDN, cache cache.ByteCache) *Web {
	token := tokens.NewTokenGenerator(config.JWT)
	return &Web{
		config: config,
		gin:    gin.New(),
		wx:     wx,
		db:     db,
		cdn:    cdn,
		cache:  cache,
		render: render.NewRender(cdn, db, true),
		token:  token,
		auth:   middleware.NewJwtAuth(token),
	}
}

func (w *Web) Identifier() string {
	return "api"
}

func (w *Web) Run() {
	logger := log.With(map[string]interface{}{
		"module": "dash",
	})
	loggerMiddleware := middleware.NewZapLoggerMiddleware(logger)
	recoveryMiddleware := middleware.NewZapRecoveryMiddleware(logger)
	//rateLimiter := middleware.NewNewRateLimiterByClientIP(100*time.Millisecond, 10, time.Hour)

	w.gin.Use(loggerMiddleware, recoveryMiddleware)

	api := w.gin.Group("/", w.auth.NewJwtAuthMiddleware(false))

	w.bindAuthRoute(api)
	w.bindUserRoute(api)
	w.bindSystemRoute(api)

	err := w.gin.Run(w.config.Address)
	if err != nil {
		log.Warnw("api server run error", field.Error(err))
	}
}

func (w *Web) uploadRemoteImage(ctx *gin.Context, url string) (string, error) {
	key := w.cdn.KeyFromURL(url)
	if key == "" {
		return key, nil
	}
	if !(strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
		return key, nil
	}
	id, err := w.auth.GetCurrentID(ctx)
	if err != nil {
		return "", err
	}
	key = qniu.DefaultKeyBuilder(strconv.Itoa(id))(url, "user")
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	ret, err := w.cdn.UploadFile(ctx, resp.Body, resp.ContentLength, key)
	if err != nil {
		return "", err
	}
	return ret.Key, nil
}
