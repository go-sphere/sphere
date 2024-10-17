package render

import (
	"github.com/samber/lo"
	"github.com/tbxark/sphere/internal/pkg/database/ent"
	"github.com/tbxark/sphere/pkg/utils/secure"
	"golang.org/x/net/context"
)

type User struct {
	ID       int    `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
	Phone    string `json:"phone,omitempty"`
}

type UserWithPlatform struct {
	*User
	Platforms []*ent.UserPlatform `json:"platforms,omitempty"`
}

func (r *Render) Me(u *ent.User) *User {
	if u == nil {
		return nil
	}
	return &User{
		ID:       u.ID,
		Username: u.Username,
		Avatar:   r.cdn.GenerateImageURL(u.Avatar, ImageWidthForAvatar),
		Phone:    u.Phone,
	}
}

func (r *Render) User(u *ent.User) *User {
	if u == nil {
		return nil
	}
	return &User{
		ID:       u.ID,
		Username: u.Username,
		Avatar:   r.cdn.GenerateImageURL(u.Avatar, ImageWidthForAvatar),
	}
}

func (r *Render) CensorUser(u *ent.User) *User {
	if u == nil {
		return nil
	}
	user := r.User(u)
	user.Username = secure.CensorString(u.Username, 5)
	return user
}

func (r *Render) UserWithPlatform(ctx context.Context, u *ent.User) *UserWithPlatform {
	if u == nil {
		return nil
	}
	plat, err := r.db.GetUserPlatforms(ctx, []int{u.ID})
	if err != nil {
		return nil
	}
	res := &UserWithPlatform{
		User: r.User(u),
	}
	res.Platforms = plat[u.ID]
	return res
}

func (r *Render) UsersWithPlatforms(ctx context.Context, list []*ent.User) []*UserWithPlatform {
	plats, err := r.db.GetUserPlatforms(ctx, lo.Map(list, func(u *ent.User, i int) int {
		return u.ID
	}))
	if err != nil {
		return nil
	}
	return lo.Map(list, func(u *ent.User, i int) *UserWithPlatform {
		res := &UserWithPlatform{
			User: r.User(u),
		}
		res.Platforms = plats[u.ID]
		return res
	})
}
