package dash

import (
	"context"

	"github.com/TBXark/sphere/database/mapper"
	{{.ServicePackage}} "{{.BizPackagePath}}/api/{{.PackagePath}}"
    "{{.BizPackagePath}}/internal/pkg/render"
)

var _ {{.ServicePackage}}.{{.ServiceName}}ServiceHTTPServer = (*Service)(nil)

func (s *Service) {{.ServiceName}}Create(ctx context.Context, request *{{.ServicePackage}}.{{.ServiceName}}CreateRequest) (*{{.ServicePackage}}.{{.ServiceName}}CreateResponse, error) {
	item, err := render.Create{{.ServiceName}}(s.db.{{.ServiceName}}.Create(), request.{{.ServiceName}}).Save(ctx)
	if err != nil {
		return nil, err
	}
	return &{{.ServicePackage}}.{{.ServiceName}}CreateResponse{
		{{.ServiceName}}: s.render.{{.ServiceName}}(item),
	}, nil
}

func (s *Service) {{.ServiceName}}Delete(ctx context.Context, request *{{.ServicePackage}}.{{.ServiceName}}DeleteRequest) (*{{.ServicePackage}}.{{.ServiceName}}DeleteResponse, error) {
	err := s.db.{{.ServiceName}}.DeleteOneID(request.Id).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return &{{.ServicePackage}}.{{.ServiceName}}DeleteResponse{}, nil
}

func (s *Service) {{.ServiceName}}Detail(ctx context.Context, request *{{.ServicePackage}}.{{.ServiceName}}DetailRequest) (*{{.ServicePackage}}.{{.ServiceName}}DetailResponse, error) {
	item, err := s.db.{{.ServiceName}}.Get(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	return &{{.ServicePackage}}.{{.ServiceName}}DetailResponse{
		{{.ServiceName}}: s.render.{{.ServiceName}}(item),
	}, nil
}

func (s *Service) {{.ServiceName}}List(ctx context.Context, request *{{.ServicePackage}}.{{.ServiceName}}ListRequest) (*{{.ServicePackage}}.{{.ServiceName}}ListResponse, error) {
	query := s.db.{{.ServiceName}}.Query()
	count, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}
	page, size := mapper.Page(count, int(request.PageSize), mapper.DefaultPageSize)
	all, err := query.Clone().Limit(size).Offset(size * int(request.Page)).All(ctx)
	if err != nil {
		return nil, err
	}
	return &{{.ServicePackage}}.{{.ServiceName}}ListResponse{
		{{.ServiceName}}s: mapper.Map(all, s.render.{{.ServiceName}}),
		TotalSize: int64(count),
		TotalPage: int64(page),
	}, nil
}

func (s *Service) {{.ServiceName}}Update(ctx context.Context, request *{{.ServicePackage}}.{{.ServiceName}}UpdateRequest) (*{{.ServicePackage}}.{{.ServiceName}}UpdateResponse, error) {
	item, err := render.UpdateOne{{.ServiceName}}(
		s.db.{{.ServiceName}}.UpdateOneID(request.{{.ServiceName}}.Id),
		request.{{.ServiceName}},
	).Save(ctx)
	if err != nil {
		return nil, err
	}
	return &{{.ServicePackage}}.{{.ServiceName}}UpdateResponse{
		{{.ServiceName}}: s.render.{{.ServiceName}}(item),
	}, nil
}