package meilisearch

import (
	"context"
	"fmt"
	"time"

	"github.com/go-sphere/sphere/search"
	"github.com/meilisearch/meilisearch-go"
)

// Config holds the configuration parameters for connecting to Meilisearch server.
type Config struct {
	Host   string `json:"host" yaml:"host"`       // Meilisearch server host URL
	APIKey string `json:"api_key" yaml:"api_key"` // API key for authentication
}

// ServiceManager wraps the Meilisearch client shared by every Searcher built
// from it. Create it with NewServiceManager; it is safe to share and has no
// Close.
type ServiceManager struct {
	service meilisearch.ServiceManager
}

// NewServiceManager creates a new ServiceManager instance with the given configuration.
// It currently never returns an error.
// It only builds the Meilisearch client; the connection is established lazily, so connectivity
// errors surface when the service is actually used rather than at construction time. Callers that
// need an eager readiness check should perform it explicitly after construction.
func NewServiceManager(conf Config) (*ServiceManager, error) {
	client := meilisearch.New(conf.Host, meilisearch.WithAPIKey(conf.APIKey))
	return &ServiceManager{
		service: client,
	}, nil
}

// Searcher implements the search.Searcher interface for Meilisearch backend.
// It provides type-safe search operations for documents of type T, which are
// encoded to and decoded from JSON. Create it with NewSearcher.
type Searcher[T any] struct {
	service    *ServiceManager
	index      meilisearch.IndexManager
	primaryKey *string
}

// NewSearcher creates a new Searcher instance for the specified index and document type.
// The primaryKey parameter is optional and can be nil, in which case
// Meilisearch infers the primary key on the first Index call. NewSearcher does
// not contact the server or create the index; Meilisearch creates a missing
// index when documents are first added. It currently never returns an error.
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

// Index adds or replaces documents and waits for the Meilisearch task
// (WaitForTaskWithContext, 1s poll). It is not fire-and-forget. A task that
// ends failed or canceled is returned as an error. If ctx ends while waiting,
// the ctx error is returned but the task may still complete on the server.
func (s *Searcher[T]) Index(ctx context.Context, docs ...T) error {
	task, err := s.index.AddDocumentsWithContext(ctx, docs, &meilisearch.DocumentOptions{
		PrimaryKey: s.primaryKey,
	})
	if err != nil {
		return err
	}
	result, err := s.service.service.WaitForTaskWithContext(ctx, task.TaskUID, time.Second)
	if err != nil {
		return err
	}
	return taskError(result)
}

// Delete removes documents by ID and waits for the Meilisearch task
// (WaitForTaskWithContext, 1s poll). It is not fire-and-forget. Unknown IDs
// are not an error. Task failure and ctx behavior match Index.
func (s *Searcher[T]) Delete(ctx context.Context, ids ...string) error {
	task, err := s.index.DeleteDocumentsWithContext(ctx, ids, &meilisearch.DocumentOptions{})
	if err != nil {
		return err
	}
	result, err := s.service.service.WaitForTaskWithContext(ctx, task.TaskUID, time.Second)
	if err != nil {
		return err
	}
	return taskError(result)
}

// taskError inspects a finished task and returns an error when the task did not
// succeed. WaitForTaskWithContext returns (task, nil) even for failed/canceled
// terminal states, so callers must check the task status explicitly.
func taskError(task *meilisearch.Task) error {
	if task == nil || task.Status == meilisearch.TaskStatusSucceeded {
		return nil
	}
	if msg := task.Error.Message; msg != "" {
		return fmt.Errorf("meilisearch: task %d %s: %s (%s)", task.TaskUID, task.Status, msg, task.Error.Code)
	}
	return fmt.Errorf("meilisearch: task %d %s", task.TaskUID, task.Status)
}

// Search runs a query against the index. Result.Total is EstimatedTotalHits,
// not TotalHits. Filter is a passthrough Meilisearch DSL string. A zero Limit
// uses Meilisearch's default (20). Hits is nil when nothing matches. A hit
// that cannot be decoded into T fails the whole call.
func (s *Searcher[T]) Search(ctx context.Context, params search.Params) (search.Result[T], error) {
	resp, err := s.index.SearchWithContext(ctx, params.Query, &meilisearch.SearchRequest{
		Offset: int64(params.Offset),
		Limit:  int64(params.Limit),
		Filter: params.Filter,
	})
	if err != nil {
		return search.Result[T]{}, err
	}
	var hits []T
	for _, hit := range resp.Hits {
		var hitData T
		dErr := hit.DecodeInto(&hitData)
		if dErr != nil {
			return search.Result[T]{}, dErr
		}
		hits = append(hits, hitData)
	}
	return search.Result[T]{
		Hits:       hits,
		Total:      resp.EstimatedTotalHits,
		Offset:     int(resp.Offset),
		Limit:      int(resp.Limit),
		Processing: resp.ProcessingTimeMs,
	}, nil
}
