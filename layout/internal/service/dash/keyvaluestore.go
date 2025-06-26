package dash

import (
	"context"

	"github.com/TBXark/sphere/database/mapper"
	dashv1 "github.com/TBXark/sphere/layout/api/dash/v1"
	"github.com/TBXark/sphere/layout/internal/pkg/render"
)

var _ dashv1.KeyValueStoreServiceHTTPServer = (*Service)(nil)

func (s *Service) KeyValueStoreCreate(ctx context.Context, request *dashv1.KeyValueStoreCreateRequest) (*dashv1.KeyValueStoreCreateResponse, error) {
	item, err := render.CreateKeyValueStore(s.db.KeyValueStore.Create(), request.KeyValueStore).Save(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.KeyValueStoreCreateResponse{
		KeyValueStore: s.render.KeyValueStore(item),
	}, nil
}

func (s *Service) KeyValueStoreDelete(ctx context.Context, request *dashv1.KeyValueStoreDeleteRequest) (*dashv1.KeyValueStoreDeleteResponse, error) {
	err := s.db.KeyValueStore.DeleteOneID(request.Id).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.KeyValueStoreDeleteResponse{}, nil
}

func (s *Service) KeyValueStoreDetail(ctx context.Context, request *dashv1.KeyValueStoreDetailRequest) (*dashv1.KeyValueStoreDetailResponse, error) {
	item, err := s.db.KeyValueStore.Get(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	return &dashv1.KeyValueStoreDetailResponse{
		KeyValueStore: s.render.KeyValueStore(item),
	}, nil
}

func (s *Service) KeyValueStoreList(ctx context.Context, request *dashv1.KeyValueStoreListRequest) (*dashv1.KeyValueStoreListResponse, error) {
	query := s.db.KeyValueStore.Query()
	count, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}
	page, size := mapper.Page(count, int(request.PageSize), mapper.DefaultPageSize)
	all, err := query.Clone().Limit(size).Offset(size * int(request.Page)).All(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.KeyValueStoreListResponse{
		KeyValueStores: mapper.Map(all, s.render.KeyValueStore),
		TotalSize:      int64(count),
		TotalPage:      int64(page),
	}, nil
}

func (s *Service) KeyValueStoreUpdate(ctx context.Context, request *dashv1.KeyValueStoreUpdateRequest) (*dashv1.KeyValueStoreUpdateResponse, error) {
	item, err := render.UpdateOneKeyValueStore(
		s.db.KeyValueStore.UpdateOneID(request.KeyValueStore.Id),
		request.KeyValueStore,
	).Save(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.KeyValueStoreUpdateResponse{
		KeyValueStore: s.render.KeyValueStore(item),
	}, nil
}
