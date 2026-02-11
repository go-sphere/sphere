package search

import "context"

// Params defines the parameters for search operations.
// It includes query string, pagination, and filtering capabilities.
type Params struct {
	Query  string // The search query string
	Offset int    // Number of results to skip for pagination
	Limit  int    // Maximum number of results to return
	Filter string // Additional filter criteria
}

// Result represents the response from a search operation with typed documents.
// It includes the matching documents and metadata about the search.
type Result[T any] struct {
	Hits       []T   // The matching documents
	Total      int64 // Total number of matching documents
	Offset     int   // The offset used in the search
	Limit      int   // The limit used in the search
	Processing int64 // Processing time in milliseconds
}

// Searcher provides full-text search capabilities for typed documents.
// It supports indexing, deletion, and search operations with pagination and filtering.
type Searcher[T any] interface {
	// Index adds or updates documents in the search index.
	Index(ctx context.Context, docs ...T) error

	// Delete removes documents from the search index by their IDs.
	Delete(ctx context.Context, ids ...string) error

	// Search performs a search operation with the given parameters.
	// Returns matching results with pagination and metadata.
	Search(ctx context.Context, params Params) (Result[T], error)
}
