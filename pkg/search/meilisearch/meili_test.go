package meilisearch

import (
	"encoding/json"
	"github.com/meilisearch/meilisearch-go"
	"testing"
)

type Movie struct {
	ID     int      `json:"id"`
	Title  string   `json:"title"`
	Genres []string `json:"genres"`
}

func TestSearch(t *testing.T) {
	client := NewMeiliSearch(&Config{
		Host:   "http://localhost:7700",
		APIKey: "luNMnaJK6FqWGYsWFwt05aFYPYNNzVcqDxLW2PXpu4E",
	})

	index := client.Index("movies")

	// If the index 'movies' does not exist, Meilisearch creates it when you first add the documents.
	//documents := []map[string]interface{}{
	//	{"id": 1, "title": "Carol", "genres": []string{"Romance", "Drama"}},
	//	{"id": 2, "title": "Wonder Woman", "genres": []string{"Action", "Adventure"}},
	//	{"id": 3, "title": "Life of Pi", "genres": []string{"Adventure", "Drama"}},
	//	{"id": 4, "title": "Mad Max: Fury Road", "genres": []string{"Adventure", "Science Fiction"}},
	//	{"id": 5, "title": "Moana", "genres": []string{"Fantasy", "Action"}},
	//	{"id": 6, "title": "Philadelphia", "genres": []string{"Drama"}},
	//}
	//task, err := index.AddDocuments(documents)
	//if err != nil {
	//	t.Error(err)
	//}
	//t.Log(task.TaskUID)

	search, err := index.SearchRaw("P", &meilisearch.SearchRequest{
		Limit: 10,
	})
	if err != nil {
		t.Error(err)
	}

	var resp SearchResponse[Movie]
	err = json.Unmarshal(*search, &resp)
	if err != nil {
		t.Error(err)
	}

	t.Log(resp.Hits)
}
