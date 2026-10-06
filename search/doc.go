// Package search is a small typed full-text search port: Index, Delete, and
// Search with offset/limit pagination.
//
// Program against [Searcher] with a concrete document type and construct the
// only driver, search/meilisearch, at wiring time.
//
// # Usage
//
//	import (
//		"context"
//
//		"github.com/go-sphere/sphere/search"
//		"github.com/go-sphere/sphere/search/meilisearch"
//	)
//
//	type Article struct {
//		ID    int64  `json:"id"`
//		Title string `json:"title"`
//	}
//
//	manager, err := meilisearch.NewServiceManager(meilisearch.Config{
//		Host:   "http://localhost:7700",
//		APIKey: "masterKey",
//	})
//	if err != nil {
//		return err
//	}
//	var s search.Searcher[Article]
//	s, err = meilisearch.NewSearcher[Article](manager, "articles", meilisearch.PrimaryKey("id"))
//	if err != nil {
//		return err
//	}
//	if err := s.Index(ctx, Article{ID: 1, Title: "hello world"}); err != nil {
//		return err
//	}
//	res, err := s.Search(ctx, search.Params{Query: "hello", Limit: 10})
//
// # Pagination
//
// Params.Filter is passed through as a backend DSL string (Meilisearch filter
// syntax). [Result].Total is an estimate, not an exact count — Meilisearch
// reports EstimatedTotalHits and may clamp it to pagination.maxTotalHits (1000
// by default). Do not paginate solely on Total; stop when a page returns fewer
// hits than requested.
package search
