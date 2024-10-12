package render

import (
	"github.com/samber/lo"
	"github.com/tbxark/go-base-api/internal/pkg/encrypt"
	"github.com/tbxark/go-base-api/pkg/dao/ent"
	"golang.org/x/net/context"
)

type UserWithPlatform struct {
	*ent.User
	Platforms []*ent.UserPlatform `json:"platforms"`
}

func (r *Render) Me(u *ent.User) *ent.User {
	if u == nil {
		return nil
	}
	u.Remark = ""
	u.UpdatedAt = 0
	u.CreatedAt = 0
	u.Avatar = r.cdn.GenerateImageURL(u.Avatar, ImageWidthForAvatar)
	return u
}

func (r *Render) User(u *ent.User) *ent.User {
	if u == nil {
		return nil
	}
	if r.hidePrivacy {
		u.Phone = ""
		u.Remark = ""
		u.CreatedAt = 0
		u.UpdatedAt = 0
		u.Flags = 0
	}
	u.Avatar = r.cdn.GenerateImageURL(u.Avatar, ImageWidthForAvatar)
	return u
}

func (r *Render) CensorUser(u *ent.User) *ent.User {
	if u == nil {
		return nil
	}
	u = r.User(u)
	u.Username = encrypt.CensorString(u.Username, 5)
	return u
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
