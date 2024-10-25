package render

import (
	"github.com/tbxark/sphere/api/entpb"
	"github.com/tbxark/sphere/internal/pkg/database/ent"
)

func (r *Render) AdminBase(a *ent.Admin) *entpb.Admin {
	if a == nil {
		return nil
	}
	return &entpb.Admin{
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

func (r *Render) AdminFull(a *ent.Admin) *entpb.Admin {
	return &entpb.Admin{
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
