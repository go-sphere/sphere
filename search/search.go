package search

import "context"

type Params struct {
	Query  string
	Offset int
	Limit  int
	Filter string
}

type Result[T any] struct {
	Hits       []T
	Total      int64
	Offset     int
	Limit      int
	Processing int64
}

type Searcher[T any] interface {
	Index(ctx context.Context, docs ...T) error
	Delete(ctx context.Context, ids ...string) error
	Search(ctx context.Context, params Params) (*Result[T], error)
}
