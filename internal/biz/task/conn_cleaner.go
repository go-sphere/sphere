package task

import (
	"github.com/tbxark/go-base-api/pkg/dao/ent"
	"golang.org/x/sync/errgroup"
)

type ConnectCleaner struct {
	db *ent.Client
}

func NewCleaner(db *ent.Client) *ConnectCleaner {
	return &ConnectCleaner{db: db}
}

func (c *ConnectCleaner) Clean() error {
	group := errgroup.Group{}
	group.Go(c.db.Close)
	return group.Wait()
}
