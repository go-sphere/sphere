package task

import (
	"github.com/tbxark/go-base-api/internal/pkg/database/ent"
	"golang.org/x/sync/errgroup"
)

type ConnectCleaner struct {
	db *ent.Client
}

func NewConnectCleaner(db *ent.Client) *ConnectCleaner {
	return &ConnectCleaner{db: db}
}

func (c *ConnectCleaner) Clean() error {
	group := errgroup.Group{}
	group.Go(c.db.Close)
	return group.Wait()
}
