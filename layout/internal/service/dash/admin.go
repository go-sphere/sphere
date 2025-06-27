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

func (s *Service) AdminCreate(ctx context.Context, request *dashv1.AdminCreateRequest) (*dashv1.AdminCreateResponse, error) {
	request.Admin.Avatar = s.storage.ExtractKeyFromURL(request.Admin.Avatar)
	u, err := render.CreateAdmin(s.db.Admin.Create(), request.Admin).Save(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.AdminCreateResponse{
		Admin: s.render.Admin(u),
	}, nil
}

func (s *Service) AdminDelete(ctx context.Context, request *dashv1.AdminDeleteRequest) (*dashv1.AdminDeleteResponse, error) {
	value, err := s.GetCurrentID(ctx)
	if err != nil {
		return nil, err
	}
	if value == request.Id {
		return nil, dashv1.AdminError_ADMIN_CANNOT_DELETE_SELF
	}
	err = s.db.Admin.DeleteOneID(request.Id).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.AdminDeleteResponse{}, nil
}

func (s *Service) AdminDetail(ctx context.Context, request *dashv1.AdminDetailRequest) (*dashv1.AdminDetailResponse, error) {
	adm, err := s.db.Admin.Get(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	return &dashv1.AdminDetailResponse{
		Admin: s.render.Admin(adm),
	}, nil
}

func (s *Service) AdminList(ctx context.Context, request *dashv1.AdminListRequest) (*dashv1.AdminListResponse, error) {
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
	return &dashv1.AdminListResponse{
		Admins:    mapper.Map(all, s.render.Admin),
		TotalSize: int64(count),
		TotalPage: int64(page),
	}, nil
}

func (s *Service) AdminUpdate(ctx context.Context, req *dashv1.AdminUpdateRequest) (*dashv1.AdminUpdateResponse, error) {
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
	return &dashv1.AdminUpdateResponse{
		Admin: s.render.Admin(u),
	}, nil
}

func (s *Service) AdminRoleList(ctx context.Context, request *dashv1.AdminRoleListRequest) (*dashv1.AdminRoleListResponse, error) {
	return &dashv1.AdminRoleListResponse{
		Roles: []string{
			PermissionAll,
			PermissionAdmin,
		},
	}, nil
}
