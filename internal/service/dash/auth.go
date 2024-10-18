package dash

import (
	"context"
	dashv1 "github.com/tbxark/sphere/api/dash/v1"
	"github.com/tbxark/sphere/internal/pkg/database/ent"
	"github.com/tbxark/sphere/internal/pkg/database/ent/admin"
	"github.com/tbxark/sphere/pkg/utils/secure"
	"github.com/tbxark/sphere/pkg/web/ginx"
	"github.com/tbxark/sphere/pkg/web/statuserr"
	"strconv"
	"time"
)

var _ dashv1.AuthServiceHTTPServer = (*Service)(nil)

type AdminToken struct {
	Admin        *ent.Admin
	AccessToken  string
	RefreshToken string
	Expires      string
}

type AdminLoginResponseWrapper = ginx.DataResponse[AdminToken]

func (s *Service) createToken(u *ent.Admin) (*AdminToken, error) {
	id := strconv.Itoa(int(u.ID))
	token, err := s.Authorizer.GenerateSignedToken(id, u.Username, u.Roles...)
	if err != nil {
		return nil, err
	}
	refresh, err := s.Authorizer.GenerateRefreshToken(id)
	if err != nil {
		return nil, err
	}
	u.Avatar = s.Storage.GenerateImageURL(u.Avatar, 512)
	return &AdminToken{
		Admin:        u,
		AccessToken:  token.Token,
		RefreshToken: refresh.Token,
		Expires:      token.ExpiresAt.Format(time.DateTime),
	}, nil
}

func (s *Service) AuthLogin(ctx context.Context, req *dashv1.AuthLoginRequest) (*dashv1.AuthLoginResponse, error) {
	u, err := s.DB.Admin.Query().Where(admin.UsernameEQ(req.Username)).Only(ctx)
	if err != nil {
		return nil, err
	}
	if !secure.IsPasswordMatch(req.Password, u.Password) {
		return nil, statuserr.NewError(400, "password not match")
	}
	token, err := s.createToken(u)
	if err != nil {
		return nil, err
	}
	return &dashv1.AuthLoginResponse{
		Avatar:       token.Admin.Avatar,
		Username:     token.Admin.Username,
		Nickname:     token.Admin.Nickname,
		Roles:        token.Admin.Roles,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expires:      token.Expires,
	}, nil

}

func (s *Service) AuthRefresh(ctx context.Context, req *dashv1.AuthRefreshRequest) (*dashv1.AuthRefreshResponse, error) {
	claims, err := s.Authorizer.ParseToken(req.RefreshToken)
	if err != nil {
		return nil, err
	}
	id, err := strconv.Atoi(claims.Subject)
	if err != nil {
		return nil, err
	}
	u, err := s.DB.Admin.Get(ctx, int64(id))
	if err != nil {
		return nil, err
	}
	token, err := s.createToken(u)
	if err != nil {
		return nil, err
	}
	return &dashv1.AuthRefreshResponse{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expires:      token.Expires,
	}, nil
}
