package api

import (
	"context"
	"fmt"
	apiv1 "github.com/tbxark/sphere/api/api/v1"
	"github.com/tbxark/sphere/internal/pkg/consts"
	"github.com/tbxark/sphere/internal/pkg/dao"
	"github.com/tbxark/sphere/internal/pkg/database/ent"
	"github.com/tbxark/sphere/internal/pkg/database/ent/userplatform"
	"github.com/tbxark/sphere/pkg/server/auth/authorizer"
	"time"
)

var _ apiv1.AuthServiceHTTPServer = (*Service)(nil)

const (
	AppTokenValidDuration = time.Hour * 24 * 7
)

func renderClaims(user *ent.User, pla *ent.UserPlatform, duration time.Duration) *authorizer.RBACClaims[int64] {
	return &authorizer.RBACClaims[int64]{
		UID:       user.ID,
		Subject:   pla.PlatformID,
		Roles:     nil,
		ExpiresAt: time.Now().Add(duration).Unix(),
	}
}

type userContext struct {
	isNew    bool
	user     *ent.User
	platform *ent.UserPlatform
}

func (s *Service) AuthWxMini(ctx context.Context, req *apiv1.AuthWxMiniRequest) (*apiv1.AuthWxMiniResponse, error) {
	wxUser, err := s.Wechat.Auth(req.Code)
	if err != nil {
		return nil, err
	}

	res, err := dao.WithTx[userContext](ctx, s.DB.Client, func(ctx context.Context, client *ent.Client) (*userContext, error) {
		userPlat, e := client.UserPlatform.Query().
			Where(userplatform.PlatformEQ(consts.WechatMiniPlatform), userplatform.PlatformIDEQ(wxUser.OpenID)).
			Only(ctx)
		// 用户存在
		if e == nil && userPlat != nil {
			u, ue := client.User.Get(ctx, userPlat.UserID)
			if ue != nil {
				return nil, ue
			}
			return &userContext{
				user:     u,
				platform: userPlat,
			}, nil
		}
		// 其他错误
		if !ent.IsNotFound(e) {
			return nil, e
		}
		// 用户不存在
		newUser, e := client.User.Create().
			SetUsername(fmt.Sprintf("微信用户%d", time.Now().Unix()/1000)).
			SetAvatar(consts.DefaultUserAvatar).
			Save(ctx)
		if e != nil {
			return nil, e
		}
		userPlat, e = client.UserPlatform.Create().SetUserID(newUser.ID).
			SetPlatform(consts.WechatMiniPlatform).
			SetPlatformID(wxUser.OpenID).
			SetSecondID(wxUser.UnionID).
			Save(ctx)
		if e != nil {
			return nil, e
		}
		return &userContext{
			isNew:    true,
			user:     newUser,
			platform: userPlat,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	token, err := s.Authorizer.GenerateToken(renderClaims(res.user, res.platform, AppTokenValidDuration))
	if err != nil {
		return nil, err
	}
	return &apiv1.AuthWxMiniResponse{
		IsNew: res.isNew,
		Token: token,
		User:  s.Render.Me(res.user),
	}, nil
}
