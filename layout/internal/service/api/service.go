package api

import (
	"net/http"
	"time"

	"github.com/TBXark/sphere/cache"
	"github.com/TBXark/sphere/layout/internal/pkg/dao"
	"github.com/TBXark/sphere/layout/internal/pkg/render"
	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/TBXark/sphere/social/wechat"
	"github.com/TBXark/sphere/storage"
)

type TokenAuthorizer = authorizer.TokenAuthorizer[authorizer.RBACClaims[int64]]

type Service struct {
	authorizer.ContextUtils[int64]

	db         *dao.Dao
	wechat     *wechat.Wechat
	render     *render.Render
	httpClient *http.Client

	cache      cache.ByteCache
	storage    storage.CDNStorage
	authorizer TokenAuthorizer
}

func NewService(db *dao.Dao, wechat *wechat.Wechat, cache cache.ByteCache, store storage.CDNStorage) *Service {
	return &Service{
		db:     db,
		wechat: wechat,
		cache:  cache,
		render: render.NewRender(db, store, true),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		storage: store,
	}
}

func (s *Service) Init(authorizer TokenAuthorizer) {
	s.authorizer = authorizer
}
