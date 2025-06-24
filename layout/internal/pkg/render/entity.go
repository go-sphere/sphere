package render

import (
	"github.com/TBXark/sphere/layout/api/entpb"
	sharedv1 "github.com/TBXark/sphere/layout/api/shared/v1"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
)

func (r *Render) AdminLite(a *ent.Admin) *entpb.Admin {
	if a == nil {
		return nil
	}
	return &entpb.Admin{
		Id:       a.ID,
		Nickname: a.Nickname,
		Avatar:   r.storage.GenerateURL(a.Avatar),
	}
}

func (r *Render) Admin(a *ent.Admin) *entpb.Admin {
	return &entpb.Admin{
		Id:        a.ID,
		Username:  a.Username,
		Nickname:  a.Nickname,
		Avatar:    r.storage.GenerateURL(a.Avatar),
		Roles:     a.Roles,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}

func (r *Render) User(u *ent.User) *sharedv1.User {
	if u == nil {
		return nil
	}
	return &sharedv1.User{
		Id:       u.ID,
		Username: u.Username,
		Avatar:   r.storage.GenerateURL(u.Avatar),
	}
}

func (r *Render) UserFull(u *ent.User) *sharedv1.User {
	if u == nil {
		return nil
	}
	return &sharedv1.User{
		Id:       u.ID,
		Username: u.Username,
		Avatar:   r.storage.GenerateURL(u.Avatar),
	}
}
