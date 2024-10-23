package dash

import (
	"context"
	dashv1 "github.com/tbxark/sphere/api/dash/v1"
	"github.com/tbxark/sphere/internal/pkg/database/ent"
	"github.com/tbxark/sphere/internal/pkg/database/ent/admin"
	"github.com/tbxark/sphere/pkg/server/auth/authorizer"
	"github.com/tbxark/sphere/pkg/server/ginx"
	"github.com/tbxark/sphere/pkg/server/statuserr"
	"github.com/tbxark/sphere/pkg/utils/secure"
	"time"
)

var _ dashv1.AuthServiceHTTPServer = (*Service)(nil)

const (
	AuthTokenValidDuration    = time.Hour * 24
	RefreshTokenValidDuration = time.Hour * 24 * 30
)

type AdminToken struct {
	Admin        *ent.Admin
	AccessToken  string
	RefreshToken string
	Expires      string
}

type AdminLoginResponseWrapper = ginx.DataResponse[AdminToken]

func renderClaims(auth authorizer.Authorizer[int64], admin *ent.Admin, duration time.Duration) *authorizer.Claims[int64] {
	return &authorizer.Claims[int64]{
		UID:       admin.ID,
		Subject:   admin.Username,
		Roles:     auth.GenerateRoles(admin.Roles),
		ExpiresAt: time.Now().Add(duration).Unix(),
	}
}

func (s *Service) createToken(u *ent.Admin) (*AdminToken, error) {
	token, err := s.Authorizer.GenerateToken(renderClaims(s.Authorizer, u, AuthTokenValidDuration))
	if err != nil {
		return nil, err
	}
	refresh, err := s.AuthRefresher.GenerateToken(renderClaims(s.Authorizer, u, RefreshTokenValidDuration))
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
	claims, err := s.AuthRefresher.ParseToken(req.RefreshToken)
	if err != nil {
		return nil, err
	}
	u, err := s.DB.Admin.Get(ctx, claims.UID)
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
