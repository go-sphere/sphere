package auth

import (
	"context"
	"github.com/TBXark/sphere/layout/internal/pkg/dao"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/userplatform"
	"github.com/TBXark/sphere/server/auth/authorizer"
	"time"
)

const (
	PlatformWechatMini = "wechat_mini"
	PlatformPhone      = "phone"
)

const (
	DefaultUserAvatar = "assets/1714097313_40109ed253fc6fa7c73df01ebe093c07.jpg"
)

const (
	AppTokenValidDuration = time.Hour * 24 * 7
)

func RenderClaims(user *ent.User, pla *ent.UserPlatform, duration time.Duration) *authorizer.RBACClaims[int64] {
	return authorizer.NewRBACClaims(user.ID, user.Username, []string{}, time.Now().Add(duration))
}

type Response struct {
	IsNew    bool
	User     *ent.User
	Platform *ent.UserPlatform
}

type Options struct {
	onCreateUser     func(user *ent.UserCreate) *ent.UserCreate
	onCreatePlatform func(platform *ent.UserPlatformCreate) *ent.UserPlatformCreate
}

type Option func(*Options)

func WithOnCreateUser(f func(user *ent.UserCreate) *ent.UserCreate) Option {
	return func(opts *Options) {
		opts.onCreateUser = f
	}
}

func WithOnCreatePlatform(f func(platform *ent.UserPlatformCreate) *ent.UserPlatformCreate) Option {
	return func(opts *Options) {
		opts.onCreatePlatform = f
	}
}

func Auth(ctx context.Context, db *dao.Dao, platformID, platformType string, options ...Option) (*Response, error) {
	opt := &Options{}
	for _, o := range options {
		o(opt)
	}
	return dao.WithTx[Response](ctx, db.Client, func(ctx context.Context, client *ent.Client) (*Response, error) {
		userPlat, err := client.UserPlatform.Query().
			Where(
				userplatform.PlatformEQ(platformType),
				userplatform.PlatformIDEQ(platformID),
			).
			Only(ctx)
		// 用户存在
		if err == nil && userPlat != nil {
			u, ue := client.User.Get(ctx, userPlat.UserID)
			if ue != nil {
				return nil, ue
			}
			return &Response{
				User:     u,
				Platform: userPlat,
			}, nil
		}
		// 其他错误
		if !ent.IsNotFound(err) {
			return nil, err
		}
		// 用户不存在
		userCreate := client.User.Create().SetAvatar(DefaultUserAvatar)
		if opt.onCreateUser != nil {
			userCreate = opt.onCreateUser(userCreate)
		}
		newUser, err := userCreate.Save(ctx)
		if err != nil {
			return nil, err
		}
		userPlatCreate := client.UserPlatform.Create().
			SetUserID(newUser.ID).
			SetPlatform(platformType).
			SetPlatformID(platformID)
		if opt.onCreatePlatform != nil {
			userPlatCreate = opt.onCreatePlatform(userPlatCreate)
		}
		userPlat, err = userPlatCreate.Save(ctx)
		if err != nil {
			return nil, err
		}
		return &Response{
			IsNew:    true,
			User:     newUser,
			Platform: userPlat,
		}, nil
	})
}
