package auth

import (
	"context"
	"errors"
	"time"

	"github.com/TBXark/sphere/layout/internal/pkg/dao"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/predicate"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/userplatform"
	"github.com/TBXark/sphere/server/auth/authorizer"
)

const (
	PlatformWechatMini = "wechat_mini"
	PlatformPhone      = "phone"
)

const (
	AppTokenValidDuration = time.Hour * 24 * 7
)

func RenderClaims(user *ent.User, pla *ent.UserPlatform, duration time.Duration) *authorizer.RBACClaims[int64] {
	return authorizer.NewRBACClaims(
		user.ID,
		pla.Platform+":"+pla.PlatformID,
		[]string{},
		time.Now().Add(duration),
	)
}

type Response struct {
	IsNew    bool
	User     *ent.User
	Platform *ent.UserPlatform
}

type (
	BeforeCreateFunc = func(ctx context.Context, client *ent.Client) error
	AfterCreateFunc  = func(ctx context.Context, client *ent.Client, user *ent.User, platform *ent.UserPlatform) error
)

type Mode int

const (
	CreateIfNotExist Mode = iota
	CreateWithoutCheck
	LoginIfExist
)

type Options struct {
	mode                 Mode
	throwOnNotFound      bool // 是否在登录时抛出用户不存在的错误
	ignorePlatformIDCase bool // 是否忽略平台ID的大小写
	beforeCreate         BeforeCreateFunc
	afterCreate          AfterCreateFunc
	onCreateUser         func(user *ent.UserCreate) *ent.UserCreate
	onCreatePlatform     func(platform *ent.UserPlatformCreate) *ent.UserPlatformCreate
}

type Option func(*Options)

func WithAuthMode(mode Mode) Option {
	return func(opts *Options) {
		opts.mode = mode
	}
}

func IgnorePlatformIDCase() Option {
	return func(opts *Options) {
		opts.ignorePlatformIDCase = true
	}
}

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

func WithBeforeCreate(f BeforeCreateFunc) Option {
	return func(opts *Options) {
		opts.beforeCreate = f
	}
}

func WithAfterCreate(f AfterCreateFunc) Option {
	return func(opts *Options) {
		opts.afterCreate = f
	}
}

func login(ctx context.Context, client *ent.Client, platformID, platformType string, opt *Options) (*Response, error) {
	userPlatPred := []predicate.UserPlatform{
		userplatform.PlatformEQ(platformType),
	}
	if opt.ignorePlatformIDCase {
		userPlatPred = append(userPlatPred, userplatform.PlatformIDEqualFold(platformID))
	} else {
		userPlatPred = append(userPlatPred, userplatform.PlatformIDEQ(platformID))
	}
	userPlat, err := client.UserPlatform.Query().
		Where(userPlatPred...).
		Only(ctx)
	if err != nil {
		if !opt.throwOnNotFound && ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err // 其他错误
	}
	oldUser, err := client.User.Get(ctx, userPlat.UserID) // 用户存在
	if err != nil {
		return nil, err // 平台存在用户不存在的话是不可能的
	}
	return &Response{
		User:     oldUser,
		Platform: userPlat,
	}, nil
}

func create(ctx context.Context, client *ent.Client, platformID, platformType string, opt *Options) (*Response, error) {
	if opt.beforeCreate != nil {
		if bErr := opt.beforeCreate(ctx, client); bErr != nil {
			return nil, bErr
		}
	}
	userCreate := client.User.Create()
	// 这里可以添加默认值或其他设置
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
	userPlat, err := userPlatCreate.Save(ctx)
	if err != nil {
		return nil, err
	}
	if opt.afterCreate != nil {
		if aErr := opt.afterCreate(ctx, client, newUser, userPlat); aErr != nil {
			return nil, aErr
		}
	}
	return &Response{
		IsNew:    true,
		User:     newUser,
		Platform: userPlat,
	}, nil
}

func Auth(ctx context.Context, db *dao.Dao, platformID, platformType string, options ...Option) (*Response, error) {
	opt := &Options{}
	for _, o := range options {
		o(opt)
	}
	return dao.WithTx[Response](ctx, db.Client, func(ctx context.Context, client *ent.Client) (*Response, error) {
		switch opt.mode {
		case CreateIfNotExist:
			resp, err := login(ctx, client, platformID, platformType, opt)
			if err != nil {
				return nil, err
			}
			if resp != nil {
				return resp, nil
			}
			return create(ctx, client, platformID, platformType, opt)
		case CreateWithoutCheck:
			return create(ctx, client, platformID, platformType, opt)
		case LoginIfExist:
			opt.throwOnNotFound = true
			return login(ctx, client, platformID, platformType, opt)
		default:
			return nil, errors.New("unsupported auth mode")
		}
	})
}
