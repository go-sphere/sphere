package dao

import (
	"context"
	"github.com/tbxark/go-base-api/pkg/dao/ent"
	"github.com/tbxark/go-base-api/pkg/dao/ent/user"
	"github.com/tbxark/go-base-api/pkg/dao/ent/userplatform"
)

func (d *Dao) GetUsers(ctx context.Context, ids []int) (map[int]*ent.User, error) {
	users, err := d.User.Query().Where(user.IDIn(RemoveDuplicateAndZero(ids)...)).All(ctx)
	if err != nil {
		return nil, err
	}
	userMap := make(map[int]*ent.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}
	return userMap, nil
}

func (d *Dao) GetUserPlatforms(ctx context.Context, ids []int) (map[int][]*ent.UserPlatform, error) {
	userPlatforms, err := d.UserPlatform.Query().Where(userplatform.UserIDIn(RemoveDuplicateAndZero(ids)...)).All(ctx)
	if err != nil {
		return nil, err
	}
	userPlatformMap := make(map[int][]*ent.UserPlatform)
	for _, up := range userPlatforms {
		userPlatformMap[up.UserID] = append(userPlatformMap[up.UserID], up)
	}
	return userPlatformMap, nil
}
