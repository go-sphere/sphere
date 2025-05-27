package dash

import (
	"context"
	"time"

	dashv1 "github.com/TBXark/sphere/layout/api/dash/v1"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/admin"
	"github.com/TBXark/sphere/server/auth/authorizer"
	"github.com/TBXark/sphere/server/ginx"
	"github.com/TBXark/sphere/server/statuserr"
	"github.com/TBXark/sphere/utils/secure"
)

var _ dashv1.AuthServiceHTTPServer = (*Service)(nil)

const (
	AuthTokenValidDuration    = time.Hour * 24
	RefreshTokenValidDuration = time.Hour * 24 * 30
	AuthExpiresTimeFormat     = "2006/01/02 15:04:05"
)

var ErrPasswordNotMatch = statuserr.NewError(400, "password not match")

type AdminToken struct {
	Admin        *ent.Admin
	AccessToken  string
	RefreshToken string
	Expires      string
}

type AdminLoginResponseWrapper = ginx.DataResponse[AdminToken]

func renderClaims(admin *ent.Admin, duration time.Duration) *authorizer.RBACClaims[int64] {
	return authorizer.NewRBACClaims(admin.ID, admin.Username, admin.Roles, time.Now().Add(duration))
}

func (s *Service) createToken(ctx context.Context, administrator *ent.Admin) (*AdminToken, error) {
	claims := renderClaims(administrator, AuthTokenValidDuration)
	token, err := s.authorizer.GenerateToken(ctx, claims)
	if err != nil {
		return nil, err
	}
	refresh, err := s.authRefresher.GenerateToken(ctx, renderClaims(administrator, RefreshTokenValidDuration))
	if err != nil {
		return nil, err
	}
	administrator.Avatar = s.storage.GenerateURL(administrator.Avatar)
	return &AdminToken{
		Admin:        administrator,
		AccessToken:  token,
		RefreshToken: refresh,
		Expires:      claims.ExpiresAt.Format(AuthExpiresTimeFormat),
	}, nil
}

func (s *Service) AuthLogin(ctx context.Context, req *dashv1.AuthLoginRequest) (*dashv1.AuthLoginResponse, error) {
	u, err := s.db.Admin.Query().Where(admin.UsernameEQ(req.Username)).Only(ctx)
	if err != nil {
		return nil, ErrPasswordNotMatch // 隐藏错误信息
	}
	if !secure.IsPasswordMatch(req.Password, u.Password) {
		return nil, ErrPasswordNotMatch
	}
	token, err := s.createToken(ctx, u)
	if err != nil {
		return nil, err
	}
	return &dashv1.AuthLoginResponse{
		Avatar:       "",
		Username:     u.Username,
		Roles:        u.Roles,
		Permissions:  nil,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expires:      token.Expires,
	}, nil
}

func (s *Service) AuthRefresh(ctx context.Context, request *dashv1.AuthRefreshRequest) (*dashv1.AuthRefreshResponse, error) {
	claims, err := s.authRefresher.ParseToken(ctx, request.RefreshToken)
	if err != nil {
		return nil, err
	}
	u, err := s.db.Admin.Get(ctx, claims.UID)
	if err != nil {
		return nil, err
	}
	token, err := s.createToken(ctx, u)
	if err != nil {
		return nil, err
	}

	return &dashv1.AuthRefreshResponse{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expires:      token.Expires,
	}, nil
}
