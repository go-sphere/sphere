package dash

import (
	"github.com/TBXark/sphere/internal/pkg/dao"
	"github.com/TBXark/sphere/internal/pkg/render"
	"github.com/TBXark/sphere/pkg/cache"
	"github.com/TBXark/sphere/pkg/server/auth/acl"
	"github.com/TBXark/sphere/pkg/server/auth/authorizer"
	"github.com/TBXark/sphere/pkg/storage"
	"github.com/TBXark/sphere/pkg/wechat"
	"github.com/alitto/pond/v2"
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
	Tasks   pond.ResultPool[string]

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
		Tasks:   pond.NewResultPool[string](16),
		Render:  render.NewRender(store, db, true),
		ACL:     acl.NewACL(),
	}
}

func (s *Service) Init(authorizer TokenAuthorizer, authRefresher TokenAuthorizer) {
	s.Authorizer = authorizer
	s.AuthRefresher = authRefresher
}
