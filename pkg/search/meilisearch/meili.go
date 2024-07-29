package meilisearch

import (
	"github.com/meilisearch/meilisearch-go"
)

type Config struct {
	Host   string `json:"host"`
	APIKey string `json:"api_key"`
}

type Meili struct {
	*meilisearch.Client
}

func NewMeiliSearch(config *Config) *Meili {
	client := meilisearch.NewClient(meilisearch.ClientConfig{
		Host:   config.Host,
		APIKey: config.APIKey,
	})
	return &Meili{
		Client: client,
	}
}

type SearchResponse[Hit any] struct {
	Hits               []Hit       `json:"hits"`
	EstimatedTotalHits int64       `json:"estimatedTotalHits,omitempty"`
	Offset             int64       `json:"offset,omitempty"`
	Limit              int64       `json:"limit,omitempty"`
	ProcessingTimeMs   int64       `json:"processingTimeMs"`
	Query              string      `json:"query"`
	FacetDistribution  interface{} `json:"facetDistribution,omitempty"`
	TotalHits          int64       `json:"totalHits,omitempty"`
	HitsPerPage        int64       `json:"hitsPerPage,omitempty"`
	Page               int64       `json:"page,omitempty"`
	TotalPages         int64       `json:"totalPages,omitempty"`
	FacetStats         interface{} `json:"facetStats,omitempty"`
	IndexUID           string      `json:"indexUid,omitempty"`
}
