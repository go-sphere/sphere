package render

import (
	"github.com/TBXark/sphere/layout/api/entpb"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
)

func (r *Render) AdminBase(a *ent.Admin) *entpb.Admin {
	if a == nil {
		return nil
	}
	return &entpb.Admin{
		Id:        a.ID,
		Username:  "",
		Nickname:  a.Nickname,
		Avatar:    r.storage.GenerateURL(a.Avatar),
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
		Avatar:    r.storage.GenerateURL(a.Avatar),
		Password:  "",
		Roles:     a.Roles,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}
