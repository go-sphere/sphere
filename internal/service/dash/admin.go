package dash

import (
	"context"
	"github.com/samber/lo"
	dashv1 "github.com/tbxark/sphere/api/dash/v1"
	"github.com/tbxark/sphere/internal/pkg/database/ent"
	"github.com/tbxark/sphere/pkg/server/statuserr"
	"github.com/tbxark/sphere/pkg/utils/secure"
)

var _ dashv1.AdminServiceHTTPServer = (*Service)(nil)

func (s *Service) AdminCreate(ctx context.Context, req *dashv1.AdminCreateRequest) (*dashv1.AdminCreateResponse, error) {
	if len(req.Password) > 8 {
		req.Password = secure.CryptPassword(req.Password)
	} else {
		return nil, statuserr.NewError(400, "password is too short")
	}
	u, err := s.DB.Admin.Create().
		SetAvatar(s.Storage.ExtractKeyFromURL(req.Avatar)).
		SetUsername(req.Username).
		SetNickname(req.Nickname).
		SetPassword(req.Password).
		SetRoles(req.Roles).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.AdminCreateResponse{
		Admin: s.Render.AdminWithRoles(u),
	}, nil
}

func (s *Service) AdminDelete(ctx context.Context, req *dashv1.AdminDeleteRequest) (*dashv1.AdminDeleteResponse, error) {
	adm, err := s.DB.Admin.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	value, err := s.Auth.GetCurrentUsername(ctx)
	if err != nil {
		return nil, err
	}
	if adm.Username == value {
		return nil, statuserr.NewError(400, "can not delete self")
	}
	err = s.DB.Admin.DeleteOneID(adm.ID).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.AdminDeleteResponse{}, nil
}

func (s *Service) AdminDetail(ctx context.Context, req *dashv1.AdminDetailRequest) (*dashv1.AdminDetailResponse, error) {
	adm, err := s.DB.Admin.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &dashv1.AdminDetailResponse{
		Admin: s.Render.AdminWithRoles(adm),
	}, nil
}

func (s *Service) AdminList(ctx context.Context, req *dashv1.AdminListRequest) (*dashv1.AdminListResponse, error) {
	all, err := s.DB.Admin.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.AdminListResponse{
		Admins: lo.Map(all, func(admin *ent.Admin, i int) *dashv1.Admin {
			return s.Render.AdminWithRoles(admin)
		}),
	}, nil
}

func (s *Service) AdminUpdate(ctx context.Context, req *dashv1.AdminUpdateRequest) (*dashv1.AdminUpdateResponse, error) {
	update := s.DB.Admin.UpdateOneID(req.Id).
		SetAvatar(s.Storage.ExtractKeyFromURL(req.Avatar)).
		SetUsername(req.Username).
		SetNickname(req.Nickname).
		SetRoles(req.Roles)
	if req.Password != "" {
		update = update.SetPassword(secure.CryptPassword(req.Password))
		if len(req.Password) > 8 {
			req.Password = secure.CryptPassword(req.Password)
		} else {
			return nil, statuserr.NewError(400, "password is too short")
		}
	}
	u, err := update.Save(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.AdminUpdateResponse{
		Admin: s.Render.AdminWithRoles(u),
	}, nil
}
