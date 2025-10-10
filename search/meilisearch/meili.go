package meilisearch

import (
	"context"
	"time"

	"github.com/go-sphere/sphere/search"
	"github.com/meilisearch/meilisearch-go"
)

// Config holds the configuration parameters for connecting to Meilisearch server.
type Config struct {
	Host   string `json:"host"`    // Meilisearch server host URL
	APIKey string `json:"api_key"` // API key for authentication
}

// ServiceManager wraps the Meilisearch service manager to provide connection management.
type ServiceManager struct {
	service meilisearch.ServiceManager
}

// NewServiceManager creates a new ServiceManager instance with the given configuration.
// It establishes a connection to the Meilisearch server and returns an error if connection fails.
func NewServiceManager(config *Config) (*ServiceManager, error) {
	client, err := meilisearch.Connect(config.Host, meilisearch.WithAPIKey(config.APIKey))
	if err != nil {
		return nil, err
	}
	return &ServiceManager{
		service: client,
	}, nil
}

// Searcher implements the search.Searcher interface for Meilisearch backend.
// It provides type-safe search operations for documents of type T.
type Searcher[T any] struct {
	service    *ServiceManager
	index      meilisearch.IndexManager
	primaryKey *string
}

// NewSearcher creates a new Searcher instance for the specified index and document type.
// The primaryKey parameter is optional and can be nil if not needed.
func NewSearcher[T any](service *ServiceManager, indexName string, primaryKey *string) (*Searcher[T], error) {
	index := service.service.Index(indexName)
	return &Searcher[T]{
		service:    service,
		index:      index,
		primaryKey: primaryKey,
	}, nil
}

// PrimaryKey is a helper function that converts a string to a pointer.
// It returns nil if the value is empty, otherwise returns a pointer to the string.
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
		dErr := hit.DecodeInto(&hitData)
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
