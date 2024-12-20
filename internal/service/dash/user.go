package dash

import (
	"context"
	dashv1 "github.com/TBXark/sphere/api/dash/v1"
	"strconv"
)

var _ dashv1.UserServiceHTTPServer = (*Service)(nil)

func (s *Service) UserInfo(ctx context.Context, req *dashv1.UserInfoRequest) (*dashv1.UserInfoResponse, error) {
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
	return &dashv1.UserInfoResponse{
		Avatar:   u.Avatar,
		RealName: u.Nickname,
		Roles:    u.Roles,
		UserId:   strconv.Itoa(int(u.ID)),
		Username: u.Username,
		Desc:     "",
		HomePath: "",
		Token:    token.AccessToken,
	}, nil
}
