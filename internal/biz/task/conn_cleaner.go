package task

import (
	"context"
	"github.com/tbxark/sphere/internal/pkg/database/ent"
	"golang.org/x/sync/errgroup"
)

type ConnectCleaner struct {
	db *ent.Client
}

func NewConnectCleaner(db *ent.Client) *ConnectCleaner {
	return &ConnectCleaner{db: db}
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
	return group.Wait()
}
