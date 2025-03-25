package dash

import (
	"github.com/TBXark/sphere/cache"
	"github.com/TBXark/sphere/layout/internal/pkg/dao"
	"github.com/TBXark/sphere/layout/internal/pkg/render"
	"github.com/TBXark/sphere/server/auth/acl"
	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/TBXark/sphere/storage"
	"github.com/TBXark/sphere/wechat"
	"github.com/alitto/pond/v2"
)

const (
	PermissionAll   = "all"
	PermissionAdmin = "admin"
)

type TokenAuthorizer = authorizer.TokenAuthorizer[authorizer.RBACClaims[int64]]

type Service struct {
	authorizer.ContextUtils[int64]

	db     *dao.Dao
	cache  cache.ByteCache
	wechat *wechat.Wechat
	render *render.Render
	tasks  pond.ResultPool[string]

	authorizer    TokenAuthorizer
	authRefresher TokenAuthorizer

	ACL     *acl.ACL
	Storage storage.ImageStorage
}

func NewService(db *dao.Dao, wx *wechat.Wechat, cache cache.ByteCache, store storage.ImageStorage) *Service {
	return &Service{
		db:      db,
		Storage: store,
		cache:   cache,
		wechat:  wx,
		tasks:   pond.NewResultPool[string](16),
		render:  render.NewRender(store, db, true),
		ACL:     acl.NewACL(),
	}
}

func (s *Service) Init(authorizer TokenAuthorizer, authRefresher TokenAuthorizer) {
	s.authorizer = authorizer
	s.authRefresher = authRefresher
}
