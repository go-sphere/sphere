package conncleaner

import (
	"context"
	"github.com/TBXark/sphere/cache"

	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
	"golang.org/x/sync/errgroup"
)

type ConnectCleaner struct {
	db    *ent.Client
	cache cache.ByteCache
}

func NewConnectCleaner(db *ent.Client, cache cache.ByteCache) *ConnectCleaner {
	return &ConnectCleaner{
		db:    db,
		cache: cache,
	}
}

func (c *ConnectCleaner) Identifier() string {
	return "connect_cleaner"
}

func (c *ConnectCleaner) Start(ctx context.Context) error {
	return nil
}

func (c *ConnectCleaner) Stop(ctx context.Context) error {
	group := errgroup.Group{}
	group.Go(c.db.Close)
	group.Go(c.cache.Close)
	return group.Wait()
}
