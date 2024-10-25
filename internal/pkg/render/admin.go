package render

import (
	datav1 "github.com/tbxark/sphere/api/data/v1"
	"github.com/tbxark/sphere/internal/pkg/database/ent"
)

func (r *Render) AdminBase(a *ent.Admin) *datav1.Admin {
	if a == nil {
		return nil
	}
	return &datav1.Admin{
		Id:        a.ID,
		Username:  "",
		Nickname:  a.Nickname,
		Avatar:    a.Avatar,
		Password:  "",
		Roles:     nil,
		CreatedAt: 0,
		UpdatedAt: 0,
	}
}

func (r *Render) AdminFull(a *ent.Admin) *datav1.Admin {
	return &datav1.Admin{
		Id:        a.ID,
		Username:  a.Username,
		Nickname:  a.Nickname,
		Avatar:    a.Avatar,
		Password:  "",
		Roles:     a.Roles,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}
