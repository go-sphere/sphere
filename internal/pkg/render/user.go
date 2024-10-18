package render

import (
	sharedv1 "github.com/tbxark/sphere/api/shared/v1"
	"github.com/tbxark/sphere/internal/pkg/database/ent"
)

func (r *Render) Me(u *ent.User) *sharedv1.User {
	if u == nil {
		return nil
	}
	return &sharedv1.User{
		Id:       u.ID,
		Username: u.Username,
		Avatar:   r.cdn.GenerateImageURL(u.Avatar, ImageWidthForAvatar),
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
		Avatar:   r.cdn.GenerateImageURL(u.Avatar, ImageWidthForAvatar),
	}
}
