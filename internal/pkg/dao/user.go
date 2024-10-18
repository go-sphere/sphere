package dao

import (
	"context"
	"github.com/tbxark/sphere/internal/pkg/database/ent"
	"github.com/tbxark/sphere/internal/pkg/database/ent/user"
	"github.com/tbxark/sphere/internal/pkg/database/ent/userplatform"
)

func (d *Dao) GetUsers(ctx context.Context, ids []int64) (map[int64]*ent.User, error) {
	users, err := d.User.Query().Where(user.IDIn(RemoveDuplicateAndZero(ids)...)).All(ctx)
	if err != nil {
		return nil, err
	}
	userMap := make(map[int64]*ent.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}
	return userMap, nil
}

func (d *Dao) GetUserPlatforms(ctx context.Context, ids []int64) (map[int64][]*ent.UserPlatform, error) {
	userPlatforms, err := d.UserPlatform.Query().Where(userplatform.UserIDIn(RemoveDuplicateAndZero(ids)...)).All(ctx)
	if err != nil {
		return nil, err
	}
	userPlatformMap := make(map[int64][]*ent.UserPlatform)
	for _, up := range userPlatforms {
		userPlatformMap[up.UserID] = append(userPlatformMap[up.UserID], up)
	}
	return userPlatformMap, nil
}
