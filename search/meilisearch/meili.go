package meilisearch

import (
	"context"
	"time"

	"github.com/go-sphere/sphere/search"
	"github.com/meilisearch/meilisearch-go"
)

type Config struct {
	Host   string `json:"host"`
	APIKey string `json:"api_key"`
}

type ServiceManager struct {
	service meilisearch.ServiceManager
}

func NewServiceManager(config *Config) (*ServiceManager, error) {
	client, err := meilisearch.Connect(config.Host, meilisearch.WithAPIKey(config.APIKey))
	if err != nil {
		return nil, err
	}
	return &ServiceManager{
		service: client,
	}, nil
}

type Searcher[T any] struct {
	service    *ServiceManager
	index      meilisearch.IndexManager
	primaryKey *string
}

func NewSearcher[T any](service *ServiceManager, indexName string, primaryKey *string) (*Searcher[T], error) {
	index := service.service.Index(indexName)
	return &Searcher[T]{
		service:    service,
		index:      index,
		primaryKey: primaryKey,
	}, nil
}

func PrimaryKey(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (s *Searcher[T]) Index(ctx context.Context, docs ...T) error {
	task, err := s.index.AddDocumentsWithContext(ctx, docs, s.primaryKey)
	if err != nil {
		return err
	}
	_, err = s.service.service.WaitForTaskWithContext(ctx, task.TaskUID, time.Second)
	return err
}

func (s *Searcher[T]) Delete(ctx context.Context, ids ...string) error {
	task, err := s.index.DeleteDocumentsWithContext(ctx, ids)
	if err != nil {
		return err
	}
	_, err = s.service.service.WaitForTaskWithContext(ctx, task.TaskUID, time.Second)
	return err
}

func (s *Searcher[T]) Search(ctx context.Context, params search.Params) (*search.Result[T], error) {
	resp, err := s.index.SearchWithContext(ctx, params.Query, &meilisearch.SearchRequest{
		Offset: int64(params.Offset),
		Limit:  int64(params.Limit),
		Filter: params.Filter,
	})
	if err != nil {
		return nil, err
	}
	var hits []T
	for _, hit := range resp.Hits {
		var hitData T
		dErr := hit.Decode(&hitData)
		if dErr != nil {
			return nil, dErr
		}
		hits = append(hits, hitData)
	}
	return &search.Result[T]{
		Hits:       hits,
		Total:      resp.TotalHits,
		Offset:     int(resp.Offset),
		Limit:      int(resp.Limit),
		Processing: resp.ProcessingTimeMs,
	}, nil
}
