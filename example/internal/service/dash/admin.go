package dash

import (
	"context"
	dashv2 "github.com/TBXark/sphere/example/api/dash/v1"
	"github.com/TBXark/sphere/example/api/entpb"
	"github.com/TBXark/sphere/example/internal/pkg/database/ent"
	"github.com/TBXark/sphere/server/statuserr"
	"github.com/TBXark/sphere/utils/secure"
	"github.com/samber/lo"
)

var _ dashv2.AdminServiceHTTPServer = (*Service)(nil)

func (s *Service) AdminCreate(ctx context.Context, req *dashv2.AdminCreateRequest) (*dashv2.AdminCreateResponse, error) {
	if len(req.Admin.Password) > 8 {
		req.Admin.Password = secure.CryptPassword(req.Admin.Password)
	} else {
		return nil, statuserr.NewError(400, "password is too short")
	}
	u, err := s.DB.Admin.Create().
		SetAvatar(s.Storage.ExtractKeyFromURL(req.Admin.Avatar)).
		SetUsername(req.Admin.Username).
		SetNickname(req.Admin.Nickname).
		SetPassword(req.Admin.Password).
		SetRoles(req.Admin.Roles).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv2.AdminCreateResponse{
		Admin: s.Render.AdminFull(u),
	}, nil
}

func (s *Service) AdminDelete(ctx context.Context, req *dashv2.AdminDeleteRequest) (*dashv2.AdminDeleteResponse, error) {
	adm, err := s.DB.Admin.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	value, err := s.GetCurrentUsername(ctx)
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
	return &dashv2.AdminDeleteResponse{}, nil
}

func (s *Service) AdminDetail(ctx context.Context, req *dashv2.AdminDetailRequest) (*dashv2.AdminDetailResponse, error) {
	adm, err := s.DB.Admin.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &dashv2.AdminDetailResponse{
		Admin: s.Render.AdminFull(adm),
	}, nil
}

func (s *Service) AdminList(ctx context.Context, req *dashv2.AdminListRequest) (*dashv2.AdminListResponse, error) {
	all, err := s.DB.Admin.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv2.AdminListResponse{
		Admins: lo.Map(all, func(admin *ent.Admin, i int) *entpb.Admin {
			return s.Render.AdminFull(admin)
		}),
	}, nil
}

func (s *Service) AdminUpdate(ctx context.Context, req *dashv2.AdminUpdateRequest) (*dashv2.AdminUpdateResponse, error) {
	update := s.DB.Admin.UpdateOneID(req.Id).
		SetAvatar(s.Storage.ExtractKeyFromURL(req.Admin.Avatar)).
		SetUsername(req.Admin.Username).
		SetNickname(req.Admin.Nickname).
		SetRoles(req.Admin.Roles)
	if req.Admin.Password != "" {
		req.Admin.Password = secure.CryptPassword(req.Admin.Password)
		update = update.SetPassword(req.Admin.Password)
	}
	u, err := update.Save(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv2.AdminUpdateResponse{
		Admin: s.Render.AdminFull(u),
	}, nil
}

func (s *Service) AdminRoleList(ctx context.Context, request *dashv2.AdminRoleListRequest) (*dashv2.AdminRoleListResponse, error) {
	return &dashv2.AdminRoleListResponse{
		Roles: []string{
			PermissionAll,
			PermissionAdmin,
		},
	}, nil
}
