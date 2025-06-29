package dash

import (
	"context"
	"time"

	dashv1 "github.com/TBXark/sphere/layout/api/dash/v1"
	"github.com/TBXark/sphere/layout/internal/pkg/dao"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/admin"
	"github.com/TBXark/sphere/server/auth/jwtauth"
	"github.com/TBXark/sphere/utils/secure"
	"github.com/google/uuid"
)

var _ dashv1.AuthServiceHTTPServer = (*Service)(nil)

const (
	AuthTokenValidDuration    = time.Hour
	RefreshTokenValidDuration = time.Hour * 24
	AuthExpiresTimeFormat     = "2006/01/02 15:04:05"
)

const (
	AuthContextKeyIP = "auth_ip"
	AuthContextKeyUA = "auth_ua"
)

type AdminToken struct {
	Admin        *ent.Admin
	AccessToken  string
	RefreshToken string
	Expires      string
}

type Session struct {
	UID     int64 `json:"uid"`
	Expires int64 `json:"expires"`
}

func (s *Service) createAdminToken(ctx context.Context, client *ent.Client, administrator *ent.Admin) (*AdminToken, error) {
	newUUID, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	authClaims := jwtauth.NewRBACClaims(administrator.ID, administrator.Username, administrator.Roles, time.Now().Add(AuthTokenValidDuration))
	token, err := s.authorizer.GenerateToken(ctx, authClaims)
	if err != nil {
		return nil, err
	}

	sessionExpires := time.Now().Add(RefreshTokenValidDuration)
	session := client.AdminSession.Create().
		SetUID(administrator.ID).
		SetSessionKey(newUUID.String()).
		SetExpires(sessionExpires.Unix())
	if ip, ok := ctx.Value(AuthContextKeyIP).(string); ok {
		session = session.SetIPAddress(ip)
	}
	if ua, ok := ctx.Value(AuthContextKeyUA).(string); ok {
		session = session.SetDeviceInfo(ua)
	}
	adminSession, err := session.Save(ctx)
	if err != nil {
		return nil, err
	}

	refreshClaims := jwtauth.NewRBACClaims(adminSession.ID, adminSession.SessionKey, nil, sessionExpires)
	refresh, err := s.authRefresher.GenerateToken(ctx, refreshClaims)
	if err != nil {
		return nil, err
	}

	return &AdminToken{
		Admin:        administrator,
		AccessToken:  token,
		RefreshToken: refresh,
		Expires:      authClaims.ExpiresAt.Format(AuthExpiresTimeFormat),
	}, nil
}

func (s *Service) AuthLogin(ctx context.Context, request *dashv1.AuthLoginRequest) (*dashv1.AuthLoginResponse, error) {
	token, err := dao.WithTx[AdminToken](ctx, s.db.Client, func(ctx context.Context, client *ent.Client) (*AdminToken, error) {
		administrator, err := client.Admin.Query().Where(admin.UsernameEqualFold(request.Username)).Only(ctx)
		if err != nil {
			return nil, dashv1.AuthError_AUTH_ERROR_PASSWORD_ERROR // 隐藏错误信息
		}
		if !secure.IsPasswordMatch(request.Password, administrator.Password) {
			return nil, dashv1.AuthError_AUTH_ERROR_PASSWORD_ERROR
		}
		return s.createAdminToken(ctx, client, administrator)
	})
	if err != nil {
		return nil, err
	}
	return &dashv1.AuthLoginResponse{
		Avatar:       s.storage.GenerateURL(token.Admin.Avatar),
		Username:     token.Admin.Username,
		Roles:        token.Admin.Roles,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expires:      token.Expires,
	}, nil
}

func (s *Service) AuthRefresh(ctx context.Context, request *dashv1.AuthRefreshRequest) (*dashv1.AuthRefreshResponse, error) {
	token, err := dao.WithTx[AdminToken](ctx, s.db.Client, func(ctx context.Context, client *ent.Client) (*AdminToken, error) {
		claims, err := s.authRefresher.ParseToken(ctx, request.RefreshToken)
		if err != nil {
			return nil, err
		}
		session, err := client.AdminSession.Get(ctx, claims.UID)
		if err != nil {
			return nil, err
		}
		if session.IsRevoked {
			return nil, dashv1.AdminSessionError_ADMIN_SESSION_ERROR_REVOKED
		}
		if session.Expires < time.Now().Unix() {
			return nil, dashv1.AdminSessionError_ADMIN_SESSION_ERROR_EXPIRED
		}
		if session.SessionKey != claims.Subject {
			return nil, dashv1.AdminSessionError_ADMIN_SESSION_ERROR_EXPIRED
		}
		administrator, err := client.Admin.Get(ctx, session.UID)
		if err != nil {
			return nil, err
		}
		err = client.AdminSession.UpdateOneID(session.ID).SetIsRevoked(true).Exec(ctx)
		if err != nil {
			return nil, err
		}
		return s.createAdminToken(ctx, client, administrator)
	})
	if err != nil {
		return nil, err
	}
	return &dashv1.AuthRefreshResponse{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expires:      token.Expires,
	}, nil
}
