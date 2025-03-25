package api

import (
	"net/http"
	"time"

	"github.com/TBXark/sphere/cache"
	"github.com/TBXark/sphere/layout/internal/pkg/dao"
	"github.com/TBXark/sphere/layout/internal/pkg/render"
	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/TBXark/sphere/storage"
	"github.com/TBXark/sphere/wechat"
)

type TokenAuthorizer = authorizer.TokenAuthorizer[authorizer.RBACClaims[int64]]

type Service struct {
	authorizer.ContextUtils[int64]

	db         *dao.Dao
	cache      cache.ByteCache
	wechat     *wechat.Wechat
	render     *render.Render
	authorizer TokenAuthorizer
	httpClient *http.Client

	Storage storage.ImageStorage
}

func NewService(db *dao.Dao, wx *wechat.Wechat, cache cache.ByteCache, store storage.ImageStorage) *Service {
	return &Service{
		db:     db,
		cache:  cache,
		wechat: wx,
		render: render.NewRender(store, db, true),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Storage: store,
	}
}

func (s *Service) Init(authorizer TokenAuthorizer) {
	s.authorizer = authorizer
}
