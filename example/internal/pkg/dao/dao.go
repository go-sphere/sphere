package dao

import (
	"github.com/TBXark/sphere/example/internal/pkg/database/ent"
)

type Dao struct {
	*ent.Client
}

func NewDao(client *ent.Client) *Dao {
	return &Dao{Client: client}
}
