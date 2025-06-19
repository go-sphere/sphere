package dao

import (
	"context"

	"github.com/TBXark/sphere/database/mapper"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/user"
	"github.com/TBXark/sphere/layout/internal/pkg/database/ent/userplatform"
)

func (d *Dao) GetUsers(ctx context.Context, ids []int64) (map[int64]*ent.User, error) {
	users, err := d.User.Query().Where(user.IDIn(mapper.UniqueSorted(ids)...)).All(ctx)
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
	userPlatforms, err := d.UserPlatform.Query().Where(userplatform.UserIDIn(mapper.UniqueSorted(ids)...)).All(ctx)
	if err != nil {
		return nil, err
	}
	userPlatformMap := make(map[int64][]*ent.UserPlatform)
	for _, up := range userPlatforms {
		userPlatformMap[up.UserID] = append(userPlatformMap[up.UserID], up)
	}
	return userPlatformMap, nil
}
