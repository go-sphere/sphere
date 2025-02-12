package dash

import (
	"context"
	dashv2 "github.com/TBXark/sphere/layout/api/dash/v1"
	"strconv"
)

var _ dashv2.UserServiceHTTPServer = (*Service)(nil)

func (s *Service) UserInfo(ctx context.Context, req *dashv2.UserInfoRequest) (*dashv2.UserInfoResponse, error) {
	id, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	u, err := s.DB.Admin.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	token, err := s.createToken(u)
	if err != nil {
		return nil, err
	}
	return &dashv2.UserInfoResponse{
		Avatar:		u.Avatar,
		RealName:	u.Nickname,
		Roles:		u.Roles,
		UserId:		strconv.Itoa(int(u.ID)),
		Username:	u.Username,
		Desc:		"",
		HomePath:	"",
		Token:		token.AccessToken,
	}, nil
}
