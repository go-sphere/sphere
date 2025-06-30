package dash

import (
	"context"

	"github.com/TBXark/sphere/database/mapper"
	dashv1 "github.com/TBXark/sphere/layout/api/dash/v1"
	"github.com/TBXark/sphere/layout/internal/pkg/render"
)

var _ dashv1.KeyValueStoreServiceHTTPServer = (*Service)(nil)

func (s *Service) CreateKeyValueStore(ctx context.Context, request *dashv1.CreateKeyValueStoreRequest) (*dashv1.CreateKeyValueStoreResponse, error) {
	item, err := render.CreateKeyValueStore(s.db.KeyValueStore.Create(), request.KeyValueStore).Save(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.CreateKeyValueStoreResponse{
		KeyValueStore: s.render.KeyValueStore(item),
	}, nil
}

func (s *Service) DeleteKeyValueStore(ctx context.Context, request *dashv1.DeleteKeyValueStoreRequest) (*dashv1.DeleteKeyValueStoreResponse, error) {
	err := s.db.KeyValueStore.DeleteOneID(request.Id).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.DeleteKeyValueStoreResponse{}, nil
}

func (s *Service) GetKeyValueStore(ctx context.Context, request *dashv1.GetKeyValueStoreRequest) (*dashv1.GetKeyValueStoreResponse, error) {
	item, err := s.db.KeyValueStore.Get(ctx, request.Id)
	if err != nil {
		return nil, err
	}
	return &dashv1.GetKeyValueStoreResponse{
		KeyValueStore: s.render.KeyValueStore(item),
	}, nil
}

func (s *Service) ListKeyValueStores(ctx context.Context, request *dashv1.ListKeyValueStoresRequest) (*dashv1.ListKeyValueStoresResponse, error) {
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
	return &dashv1.ListKeyValueStoresResponse{
		KeyValueStores: mapper.Map(all, s.render.KeyValueStore),
		TotalSize:      int64(count),
		TotalPage:      int64(page),
	}, nil
}

func (s *Service) UpdateKeyValueStore(ctx context.Context, request *dashv1.UpdateKeyValueStoreRequest) (*dashv1.UpdateKeyValueStoreResponse, error) {
	item, err := render.UpdateOneKeyValueStore(
		s.db.KeyValueStore.UpdateOneID(request.KeyValueStore.Id),
		request.KeyValueStore,
	).Save(ctx)
	if err != nil {
		return nil, err
	}
	return &dashv1.UpdateKeyValueStoreResponse{
		KeyValueStore: s.render.KeyValueStore(item),
	}, nil
}
