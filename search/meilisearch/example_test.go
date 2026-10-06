package meilisearch_test

import (
	"context"
	"fmt"

	"github.com/go-sphere/sphere/search"
	"github.com/go-sphere/sphere/search/meilisearch"
)

type Article struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// ExampleNewSearcher indexes and queries documents. It needs a running
// Meilisearch server, so it is compiled but not run.
func ExampleNewSearcher() {
	manager, err := meilisearch.NewServiceManager(meilisearch.Config{
		Host:   "http://localhost:7700",
		APIKey: "masterKey",
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	articles, err := meilisearch.NewSearcher[Article](manager, "articles", meilisearch.PrimaryKey("id"))
	if err != nil {
		fmt.Println(err)
		return
	}
	var s search.Searcher[Article] = articles

	ctx := context.Background()
	if err := s.Index(ctx, Article{ID: 1, Title: "hello world"}); err != nil {
		fmt.Println(err)
		return
	}
	// Page until a page comes back short; Total is only an estimate.
	const pageSize = 20
	for offset := 0; ; offset += pageSize {
		res, err := s.Search(ctx, search.Params{Query: "hello", Offset: offset, Limit: pageSize})
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, hit := range res.Hits {
			fmt.Println(hit.ID, hit.Title)
		}
		if len(res.Hits) < pageSize {
			break
		}
	}
	if err := s.Delete(ctx, "1"); err != nil {
		fmt.Println(err)
	}
}
