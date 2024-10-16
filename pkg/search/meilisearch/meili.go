package meilisearch

import (
	"github.com/meilisearch/meilisearch-go"
)

type Config struct {
	Host   string `json:"host"`
	APIKey string `json:"api_key"`
}

type Meili struct {
	meilisearch.ServiceManager
}

func NewMeiliSearch(config *Config) *Meili {
	client := meilisearch.New(config.Host, meilisearch.WithAPIKey(config.APIKey))
	return &Meili{
		ServiceManager: client,
	}
}
