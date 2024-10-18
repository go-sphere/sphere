package dash

import (
	"github.com/tbxark/sphere/internal/pkg/dao"
	"github.com/tbxark/sphere/internal/pkg/render"
	"github.com/tbxark/sphere/pkg/cache"
	"github.com/tbxark/sphere/pkg/storage"
	"github.com/tbxark/sphere/pkg/web/auth/authorizer"
	"github.com/tbxark/sphere/pkg/web/middleware/auth"
	"github.com/tbxark/sphere/pkg/wechat"
)

type Service struct {
	DB         *dao.Dao
	Storage    storage.Storage
	Cache      cache.ByteCache
	WeChat     *wechat.Wechat
	Render     *render.Render
	Authorizer authorizer.Authorizer
	Auth       *auth.Auth
	ACL        *auth.ACL
}

func NewService(db *dao.Dao, wx *wechat.Wechat, store storage.Storage, cache cache.ByteCache) *Service {
	return &Service{
		DB:      db,
		Storage: store,
		Cache:   cache,
		WeChat:  wx,
		Render:  render.NewRender(store, db, true),
		ACL:     auth.NewACL(),
	}
}

func (s *Service) Init(auth *auth.Auth, authorizer authorizer.Authorizer) {
	s.Auth = auth
	s.Authorizer = authorizer
}
