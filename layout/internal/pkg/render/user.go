package render

import (
	sharedv1 "github.com/TBXark/sphere/layout/api/shared/v1"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
)

func (r *Render) Me(u *ent.User) *sharedv1.User {
	if u == nil {
		return nil
	}
	return &sharedv1.User{
		Id:       u.ID,
		Username: u.Username,
		Avatar:   r.storage.GenerateURL(u.Avatar),
		Phone:    u.Phone,
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
