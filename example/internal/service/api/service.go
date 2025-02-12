package api

import (
	"github.com/TBXark/sphere/cache"
	"github.com/TBXark/sphere/example/internal/pkg/dao"
	"github.com/TBXark/sphere/example/internal/pkg/render"
	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/TBXark/sphere/storage"
	"github.com/TBXark/sphere/wechat"
	"net/http"
	"time"
)

type TokenAuthorizer = authorizer.TokenAuthorizer[authorizer.RBACClaims[int64]]

type Service struct {
	authorizer.ContextUtils[int64]
	DB         *dao.Dao
	Storage    storage.Storage
	Cache      cache.ByteCache
	Wechat     *wechat.Wechat
	Render     *render.Render
	Authorizer TokenAuthorizer
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

func (s *Service) Init(authorizer TokenAuthorizer) {
	s.Authorizer = authorizer
}
