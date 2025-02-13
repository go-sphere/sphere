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

/*
// 兼容vben-admin-pro的接口返回格式
export function convertKeysToCamelCase<T>(obj: T): T {
  if (Array.isArray(obj)) {
    return obj.map((item) => convertKeysToCamelCase(item)) as unknown as T;
  } else if (obj !== null && typeof obj === 'object') {
    const newObj: Record<string, unknown> = {};
    for (const key of Object.keys(obj)) {
      const newKey = key.replaceAll(/_([a-z])/g, (_, letter) =>
        letter.toUpperCase(),
      );
      newObj[newKey] = convertKeysToCamelCase(
        (obj as Record<string, unknown>)[key],
      );
    }
    return newObj as T;
  }
  return obj;
}
*/
