package dash

import (
	"context"

	"github.com/TBXark/sphere/database/bind"
	"github.com/TBXark/sphere/database/mapper"
	dashv1 "github.com/TBXark/sphere/layout/api/dash/v1"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/admin"
	"github.com/TBXark/sphere/layout/internal/pkg/render"
	"github.com/TBXark/sphere/utils/secure"
)

var _ dashv1.AdminServiceHTTPServer = (*Service)(nil)

func (s *Service) CreateAdmin(ctx context.Context, request *dashv1.CreateAdminRequest) (*dashv1.CreateAdminResponse, error) {
	request.Admin.Avatar = s.storage.ExtractKeyFromURL(request.Admin.Avatar)
	request.Admin.Password = secure.CryptPassword(request.Admin.Password)
	u, err := render.CreateAdmin(s.db.Admin.Create(), request.Admin).Save(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.CreateAdminResponse{
		Admin: s.render.Admin(u),
	}, nil
}

func (s *Service) DeleteAdmin(ctx context.Context, request *dashv1.DeleteAdminRequest) (*dashv1.DeleteAdminResponse, error) {
	value, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	if value == request.Id {
		return nil, dashv1.AdminError_ADMIN_ERROR_CANNOT_DELETE_SELF
	}
	err = s.db.Admin.DeleteOneID(request.Id).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.DeleteAdminResponse{}, nil
}

func (s *Service) GetAdmin(ctx context.Context, request *dashv1.GetAdminRequest) (*dashv1.GetAdminResponse, error) {
	adm, err := s.db.Admin.Get(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	return &dashv1.GetAdminResponse{
		Admin: s.render.Admin(adm),
	}, nil
}

func (s *Service) ListAdmins(ctx context.Context, request *dashv1.ListAdminsRequest) (*dashv1.ListAdminsResponse, error) {
	query := s.db.Admin.Query()
	count, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}
	page, size := mapper.Page(count, int(request.PageSize), mapper.DefaultPageSize)
	all, err := query.Clone().Limit(size).Offset(size * int(request.Page)).All(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.ListAdminsResponse{
		Admins:    mapper.Map(all, s.render.Admin),
		TotalSize: int64(count),
		TotalPage: int64(page),
	}, nil
}

func (s *Service) UpdateAdmin(ctx context.Context, req *dashv1.UpdateAdminRequest) (*dashv1.UpdateAdminResponse, error) {
	if req.Admin.Password != "" {
		req.Admin.Password = secure.CryptPassword(req.Admin.Password)
	}
	u, err := render.UpdateOneAdmin(
		s.db.Admin.UpdateOneID(req.Admin.Id),
		req.Admin,
		bind.IgnoreSetZeroField(admin.FieldPassword),
	).Save(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.UpdateAdminResponse{
		Admin: s.render.Admin(u),
	}, nil
}

func (s *Service) ListAdminRoles(ctx context.Context, request *dashv1.ListAdminRolesRequest) (*dashv1.ListAdminRolesResponse, error) {
	return &dashv1.ListAdminRolesResponse{
		Roles: []string{
			PermissionAll,
			PermissionAdmin,
		},
	}, nil
}
