package dao

import "github.com/tbxark/go-base-api/pkg/dao/ent"

type Database struct {
	*ent.Client
}

func NewDatabase(client *ent.Client) *Database {
	return &Database{Client: client}
}
