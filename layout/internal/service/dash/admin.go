package dash

import (
	"context"

	"github.com/TBXark/sphere/database/bind"
	"github.com/TBXark/sphere/database/mapper"
	dashv1 "github.com/TBXark/sphere/layout/api/dash/v1"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/admin"
	"github.com/TBXark/sphere/layout/internal/pkg/render"
	"github.com/TBXark/sphere/server/statuserr"
	"github.com/TBXark/sphere/utils/secure"
)

var _ dashv1.AdminServiceHTTPServer = (*Service)(nil)

func (s *Service) AdminCreate(ctx context.Context, req *dashv1.AdminCreateRequest) (*dashv1.AdminCreateResponse, error) {
	if len(req.Admin.Password) > 8 {
		req.Admin.Password = secure.CryptPassword(req.Admin.Password)
	} else {
		return nil, statuserr.BadRequestError("password is too short")
	}
	req.Admin.Avatar = s.storage.ExtractKeyFromURL(req.Admin.Avatar)
	u, err := render.CreateAdmin(s.db.Admin.Create(), req.Admin).Save(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.AdminCreateResponse{
		Admin: s.render.AdminFull(u),
	}, nil
}

func (s *Service) AdminDelete(ctx context.Context, req *dashv1.AdminDeleteRequest) (*dashv1.AdminDeleteResponse, error) {
	adm, err := s.db.Admin.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	value, err := s.GetCurrentUsername(ctx)
	if err != nil {
		return nil, err
	}
	if adm.Username == value {
		return nil, statuserr.BadRequestError("can not delete self")
	}
	err = s.db.Admin.DeleteOneID(adm.ID).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.AdminDeleteResponse{}, nil
}

func (s *Service) AdminDetail(ctx context.Context, req *dashv1.AdminDetailRequest) (*dashv1.AdminDetailResponse, error) {
	adm, err := s.db.Admin.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &dashv1.AdminDetailResponse{
		Admin: s.render.AdminFull(adm),
	}, nil
}

func (s *Service) AdminList(ctx context.Context, req *dashv1.AdminListRequest) (*dashv1.AdminListResponse, error) {
	query := s.db.Admin.Query()
	if req.PageSize == 0 {
		req.PageSize = mapper.DefaultPageSize
	}
	count, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}
	page := mapper.Page(count, int(req.PageSize), mapper.DefaultPageSize)
	all, err := query.Clone().All(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.AdminListResponse{
		Admins:    mapper.Map(all, s.render.AdminFull),
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
		Admin: s.render.AdminFull(u),
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
