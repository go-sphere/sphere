package api

import (
	"context"

	apiv1 "github.com/TBXark/sphere/layout/api/api/v1"
	"github.com/TBXark/sphere/layout/internal/pkg/auth"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/userplatform"
	"github.com/TBXark/sphere/social/wechat"
)

var _ apiv1.UserServiceHTTPServer = (*Service)(nil)

func (s *Service) GetCurrentUser(ctx context.Context, request *apiv1.GetCurrentUserRequest) (*apiv1.GetCurrentUserResponse, error) {
	id, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	me, err := s.db.User.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &apiv1.GetCurrentUserResponse{
		User: s.render.UserFull(me),
	}, nil
}

func (s *Service) ListUserPlatforms(ctx context.Context, request *apiv1.ListUserPlatformsRequest) (*apiv1.ListUserPlatformsResponse, error) {
	id, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	me, err := s.db.User.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	plat, err := s.db.UserPlatform.Query().Where(userplatform.UserIDEQ(id)).All(ctx)
	if err != nil {
		return nil, err
	}
	res := apiv1.ListUserPlatformsResponse{
		Username: me.Username,
	}
	for _, p := range plat {
		switch p.Platform {
		case auth.PlatformWechatMini:
			res.WechatMini = p.PlatformID
		case auth.PlatformPhone:
			res.Phone = p.PlatformID
		}
	}
	return &res, nil
}

func (s *Service) BindPhoneWxMini(ctx context.Context, request *apiv1.BindPhoneWxMiniRequest) (*apiv1.BindPhoneWxMiniResponse, error) {
	userId, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	number, err := s.wechat.GetUserPhoneNumber(ctx, request.Code, wechat.WithRetryable(true))
	if err != nil {
		return nil, err
	}
	if number.PhoneInfo.CountryCode != "86" {
		return nil, apiv1.AuthError_AUTH_ERROR_UNSUPPORTED_PHONE_REGION
	}
	err = s.db.UserPlatform.Create().
		SetUserID(userId).
		SetPlatform(auth.PlatformPhone).
		SetPlatformID(number.PhoneInfo.PhoneNumber).
		Exec(ctx)
	if err != nil {
		return nil, err
	}
	return &apiv1.BindPhoneWxMiniResponse{}, nil
}
