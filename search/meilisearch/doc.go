// Package meilisearch is the search.Searcher driver built on meilisearch-go.
//
// Create one [ServiceManager] per Meilisearch server with
// [NewServiceManager], then one [Searcher] per index and document type with
// [NewSearcher]. Construction does not contact the server; connectivity
// errors surface on first use. Nothing needs closing.
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
//	manager, err := meilisearch.NewServiceManager(meilisearch.Config{
//		Host:   "http://localhost:7700",
//		APIKey: "masterKey",
//	})
//	if err != nil {
//		return err
//	}
//	articles, err := meilisearch.NewSearcher[Article](manager, "articles", meilisearch.PrimaryKey("id"))
//	if err != nil {
//		return err
//	}
//	err = articles.Index(ctx, Article{ID: 1, Title: "hello world"})
//	res, err := articles.Search(ctx, search.Params{Query: "hello", Filter: "id > 0", Limit: 10})
//
// # Behavior
//
//   - Index and Delete wait for the Meilisearch task (polling every second)
//     and report failed tasks as errors; they are not fire-and-forget.
//   - Search maps Result.Total from EstimatedTotalHits, not TotalHits.
//   - Params.Filter is passed through verbatim as Meilisearch filter syntax;
//     filtering on an attribute requires it to be filterable in the index
//     settings, which this package does not manage.
package meilisearch
