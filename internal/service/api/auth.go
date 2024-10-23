package api

import (
	"context"
	"fmt"
	apiv1 "github.com/tbxark/sphere/api/api/v1"
	"github.com/tbxark/sphere/internal/pkg/consts"
	"github.com/tbxark/sphere/internal/pkg/dao"
	"github.com/tbxark/sphere/internal/pkg/database/ent"
	"github.com/tbxark/sphere/internal/pkg/database/ent/userplatform"
	"time"
)

var _ apiv1.AuthServiceHTTPServer = (*Service)(nil)

func (s *Service) AuthWxMini(ctx context.Context, req *apiv1.AuthWxMiniRequest) (*apiv1.AuthWxMiniResponse, error) {
	wxUser, err := s.Wechat.Auth(req.Code)
	if err != nil {
		return nil, err
	}

	res, err := dao.WithTx[apiv1.AuthWxMiniResponse](ctx, s.DB.Client, func(ctx context.Context, client *ent.Client) (*apiv1.AuthWxMiniResponse, error) {
		userPlat, e := client.UserPlatform.Query().
			Where(userplatform.PlatformEQ(consts.WechatMiniPlatform), userplatform.PlatformIDEQ(wxUser.OpenID)).
			Only(ctx)
		// 用户存在
		if e == nil && userPlat != nil {
			u, ue := client.User.Get(ctx, userPlat.UserID)
			if ue != nil {
				return nil, ue
			}
			return &apiv1.AuthWxMiniResponse{
				User:  s.Render.Me(u),
				IsNew: false,
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
		_, e = client.UserPlatform.Create().SetUserID(newUser.ID).
			SetPlatform(consts.WechatMiniPlatform).
			SetPlatformID(wxUser.OpenID).
			SetSecondID(wxUser.UnionID).
			Save(ctx)
		if e != nil {
			return nil, e
		}
		return &apiv1.AuthWxMiniResponse{
			User:  s.Render.Me(newUser),
			IsNew: true,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	token, err := s.Authorizer.GenerateToken(res.User.Id, consts.WechatMiniPlatform+":"+wxUser.OpenID)
	if err != nil {
		return nil, err
	}
	res.Token = token.Token
	return res, nil
}
