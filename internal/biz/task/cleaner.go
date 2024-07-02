package task

import (
	"errors"
	"github.com/tbxark/go-base-api/pkg/dao/ent"
	"github.com/tbxark/go-base-api/pkg/log"
)

type Cleaner struct {
	db *ent.Client
}

func NewCleaner(db *ent.Client) *Cleaner {
	return &Cleaner{db: db}
}

func (c *Cleaner) Clean() error {
	errs := make([]error, 0)
	if e := c.db.Close(); e != nil {
		errs = append(errs, e)
	}
	if e := log.Sync(); e != nil {
		errs = append(errs, e)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
