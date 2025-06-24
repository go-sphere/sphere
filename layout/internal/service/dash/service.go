package dash

import (
	"github.com/TBXark/sphere/cache"
	"github.com/TBXark/sphere/cache/memory"
	"github.com/TBXark/sphere/layout/internal/pkg/dao"
	"github.com/TBXark/sphere/layout/internal/pkg/render"
	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/TBXark/sphere/server/auth/jwtauth"
	"github.com/TBXark/sphere/social/wechat"
	"github.com/TBXark/sphere/storage"
	"github.com/alitto/pond/v2"
)

const (
	PermissionAll   = "all"
	PermissionAdmin = "admin"
)

type TokenAuthorizer = authorizer.TokenAuthorizer[int64, *jwtauth.RBACClaims[int64]]

type Service struct {
	authorizer.ContextUtils[int64]

	db     *dao.Dao
	wechat *wechat.Wechat
	render *render.Render

	cache   cache.ByteCache
	session cache.ByteCache
	storage storage.CDNStorage
	tasks   pond.ResultPool[string]

	authorizer    TokenAuthorizer
	authRefresher TokenAuthorizer
}

func NewService(db *dao.Dao, wechat *wechat.Wechat, cache cache.ByteCache, store storage.CDNStorage) *Service {
	return &Service{
		db:      db,
		wechat:  wechat,
		render:  render.NewRender(db, store, true),
		cache:   cache,
		session: memory.NewByteCache(),
		storage: store,
		tasks:   pond.NewResultPool[string](16),
	}
}

func (s *Service) Init(authorizer TokenAuthorizer, authRefresher TokenAuthorizer) {
	s.authorizer = authorizer
	s.authRefresher = authRefresher
}
