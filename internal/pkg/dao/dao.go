package dao

import "github.com/tbxark/go-base-api/pkg/dao/ent"

type Dao struct {
	*ent.Client
}

func NewDao(client *ent.Client) *Dao {
	return &Dao{Client: client}
}
