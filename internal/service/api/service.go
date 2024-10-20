package api

import (
	"github.com/tbxark/sphere/internal/pkg/dao"
	"github.com/tbxark/sphere/internal/pkg/render"
	"github.com/tbxark/sphere/pkg/cache"
	"github.com/tbxark/sphere/pkg/server/auth/authorizer"
	"github.com/tbxark/sphere/pkg/server/middleware/auth"
	"github.com/tbxark/sphere/pkg/storage"
	"github.com/tbxark/sphere/pkg/wechat"
	"net/http"
	"time"
)

type Service struct {
	DB         *dao.Dao
	Storage    storage.Storage
	Cache      cache.ByteCache
	Wechat     *wechat.Wechat
	Render     *render.Render
	Authorizer authorizer.Authorizer
	Auth       *auth.Auth[int64, string]
	httpClient *http.Client
}

func NewService(db *dao.Dao, wx *wechat.Wechat, store storage.Storage, cache cache.ByteCache) *Service {
	return &Service{
		DB:      db,
		Storage: store,
		Cache:   cache,
		Wechat:  wx,
		Render:  render.NewRender(store, db, true),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *Service) Init(auth *auth.Auth[int64, string], authorizer authorizer.Authorizer) {
	s.Auth = auth
	s.Authorizer = authorizer
}
