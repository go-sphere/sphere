package api

import (
	"github.com/gin-gonic/gin"
	"github.com/tbxark/sphere/internal/pkg/dao"
	"github.com/tbxark/sphere/internal/pkg/render"
	"github.com/tbxark/sphere/pkg/cache"
	"github.com/tbxark/sphere/pkg/log"
	"github.com/tbxark/sphere/pkg/log/logfields"
	"github.com/tbxark/sphere/pkg/storage"
	"github.com/tbxark/sphere/pkg/web/auth/jwtauth"
	"github.com/tbxark/sphere/pkg/web/middleware/auth"
	"github.com/tbxark/sphere/pkg/web/middleware/logger"
	"github.com/tbxark/sphere/pkg/wechat"
	"net/http"
	"strconv"
	"strings"
)

// @title API
// @version 1.0.0
// @description API docs
// @accept json
// @produce json

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @description JWT token

type Config struct {
	JWT     string `json:"jwt"`
	Address string `json:"address"`
}

type Web struct {
	config  *Config
	engine  *gin.Engine
	DB      *dao.Dao
	Storage storage.Storage
	Cache   cache.ByteCache
	Wechat  *wechat.Wechat
	Render  *render.Render
	JwtAuth *jwtauth.JwtAuth
	Auth    *auth.Auth
}

func NewWebServer(config *Config, db *dao.Dao, wx *wechat.Wechat, store storage.Storage, cache cache.ByteCache) *Web {
	token := jwtauth.NewJwtAuth(config.JWT)
	return &Web{
		config:  config,
		engine:  gin.New(),
		DB:      db,
		Storage: store,
		Cache:   cache,
		Wechat:  wx,
		Render:  render.NewRender(store, db, true),
		JwtAuth: token,
		Auth:    auth.NewAuth(jwtauth.AuthorizationPrefixBearer, token),
	}
}

func (w *Web) Identifier() string {
	return "api"
}

func (w *Web) Run() error {
	zapLogger := log.ZapLogger().With(logfields.String("module", "api"))
	loggerMiddleware := logger.NewZapLoggerMiddleware(zapLogger)
	recoveryMiddleware := logger.NewZapRecoveryMiddleware(zapLogger)
	//rateLimiter := middleware.NewNewRateLimiterByClientIP(100*time.Millisecond, 10, time.Hour)

	w.engine.Use(loggerMiddleware, recoveryMiddleware)

	api := w.engine.Group("/", w.Auth.NewAuthMiddleware(false))

	w.bindAuthRoute(api)
	w.bindUserRoute(api)
	w.bindSystemRoute(api)

	return w.engine.Run(w.config.Address)
}

func (w *Web) uploadRemoteImage(ctx *gin.Context, url string) (string, error) {
	key := w.Storage.ExtractKeyFromURL(url)
	if key == "" {
		return key, nil
	}
	if !(strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
		return key, nil
	}
	id, err := w.Auth.GetCurrentID(ctx)
	if err != nil {
		return "", err
	}
	key = storage.DefaultKeyBuilder(strconv.Itoa(id))(url, "user")
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	ret, err := w.Storage.UploadFile(ctx, resp.Body, resp.ContentLength, key)
	if err != nil {
		return "", err
	}
	return ret.Key, nil
}
