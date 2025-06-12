package api

import (
	"context"
	"fmt"

	apiv1 "github.com/TBXark/sphere/layout/api/api/v1"
	"github.com/TBXark/sphere/layout/internal/pkg/auth"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
	"github.com/TBXark/sphere/utils/idgenerator"
)

var _ apiv1.AuthServiceHTTPServer = (*Service)(nil)

func (s *Service) AuthWxMini(ctx context.Context, req *apiv1.AuthWxMiniRequest) (*apiv1.AuthWxMiniResponse, error) {
	data, err := s.wechat.JsCode2Session(ctx, req.Code)
	if err != nil {
		return nil, err
	}
	res, err := auth.Auth(
		ctx, s.db, data.OpenID, auth.PlatformWechatMini,
		auth.WithAuthMode(auth.CreateWithoutCheck),
		auth.WithOnCreateUser(func(user *ent.UserCreate) *ent.UserCreate {
			return user.SetUsername(fmt.Sprintf("wx_%d", idgenerator.NextId()))
		}),
		auth.WithOnCreatePlatform(func(platform *ent.UserPlatformCreate) *ent.UserPlatformCreate {
			return platform.SetSecondID(data.UnionID)
		}),
	)
	if err != nil {
		return nil, err
	}
	token, err := s.authorizer.GenerateToken(ctx, auth.RenderClaims(res.User, res.Platform, auth.AppTokenValidDuration))
	if err != nil {
		return nil, err
	}
	return &apiv1.AuthWxMiniResponse{
		IsNew: res.IsNew,
		Token: token,
		User:  s.render.Me(res.User),
	}, nil
}
