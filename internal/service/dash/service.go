package dash

import (
	"github.com/tbxark/sphere/internal/pkg/dao"
	"github.com/tbxark/sphere/internal/pkg/render"
	"github.com/tbxark/sphere/pkg/cache"
	"github.com/tbxark/sphere/pkg/server/auth/acl"
	"github.com/tbxark/sphere/pkg/server/auth/authorizer"
	"github.com/tbxark/sphere/pkg/storage"
	"github.com/tbxark/sphere/pkg/wechat"
)

const (
	PermissionAll   = "all"
	PermissionAdmin = "admin"
)

type TokenAuthorizer = authorizer.TokenAuthorizer[authorizer.RBACClaims[int64]]

type Service struct {
	authorizer.ContextUtils[int64]
	DB      *dao.Dao
	Storage storage.Storage
	Cache   cache.ByteCache
	WeChat  *wechat.Wechat
	Render  *render.Render

	Authorizer    TokenAuthorizer
	AuthRefresher TokenAuthorizer
	ACL           *acl.ACL
}

func NewService(db *dao.Dao, wx *wechat.Wechat, store storage.Storage, cache cache.ByteCache) *Service {
	return &Service{
		DB:      db,
		Storage: store,
		Cache:   cache,
		WeChat:  wx,
		Render:  render.NewRender(store, db, true),
		ACL:     acl.NewACL(),
	}
}

func (s *Service) Init(authorizer TokenAuthorizer, authRefresher TokenAuthorizer) {
	s.Authorizer = authorizer
	s.AuthRefresher = authRefresher
}
