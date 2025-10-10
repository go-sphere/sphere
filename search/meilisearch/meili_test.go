package meilisearch

import (
	"context"
	"testing"

	"github.com/go-sphere/sphere/search"
)

type Article struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

/*
docker run -it --rm \
  -p 7700:7700 \
  -v $(pwd)/meili_data:/meili_data \
  getmeili/meilisearch:latest
*/

func TestSearcher(t *testing.T) {
	manager, err := NewServiceManager(&Config{
		Host:   "http://localhost:7700",
		APIKey: "8IdbIxzCm86BaD8ZkT4SGv9vaipY1Ax7i0sz_Qv8wTI",
	})
	if err != nil {
		t.Skipf("Meilisearch server not available, skipping test: %v", err)
	}
	searcher, err := NewSearcher[Article](manager, "articles", PrimaryKey("id"))
	if err != nil {
		t.Errorf("Failed to create searcher: %v", err)
		return
	}

	ctx := context.Background()

	articles := []Article{
		{
			ID:      1,
			Title:   "hello world",
			Content: "This is a test article",
		},
		{
			ID:      2,
			Title:   "goodbye world",
			Content: "This is another test article",
		},
	}
	err = searcher.Index(ctx, articles...)
	if err != nil {
		t.Errorf("Indexing articles failed: %v", err)
		return
	}

	result, err := searcher.Search(ctx, search.Params{
		Query: "hello",
	})
	if err != nil {
		t.Errorf("Search failed: %v", err)
		return
	}

	if len(result.Hits) != 1 || result.Hits[0].Title != "hello world" {
		t.Errorf("Expected to find 1 article with title 'hello world', got %d articles", len(result.Hits))
		return
	}

	err = searcher.Delete(ctx, "2")
	if err != nil {
		t.Errorf("Deleting article failed: %v", err)
		return
	}

	found, err := searcher.Search(ctx, search.Params{
		Query: "goodbye world",
	})
	if err != nil {
		t.Errorf("Search after deletion failed: %v", err)
		return
	}

	if len(found.Hits) != 0 {
		t.Errorf("Expected to find no articles after deletion, got %d articles", len(found.Hits))
		return
	}
}
