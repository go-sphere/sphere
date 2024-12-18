package task

import (
	"context"
	"github.com/TBXark/sphere/internal/pkg/dao"
	"github.com/TBXark/sphere/internal/pkg/database/ent"
	"github.com/TBXark/sphere/internal/pkg/database/ent/keyvaluestore"
	"github.com/TBXark/sphere/pkg/utils/secure"
	"strconv"
	"time"
)

type DashInitialize struct {
	db *dao.Dao
}

func NewDashInitialize(db *dao.Dao) *DashInitialize {
	return &DashInitialize{db: db}
}

func initAdminIfNeed(ctx context.Context, client *ent.Client) error {
	count, err := client.Admin.Query().Count(context.Background())
	if err != nil || count > 0 {
		return nil
	}
	return client.Admin.Create().
		SetUsername("admin").
		SetPassword(secure.CryptPassword("aA1234567")).
		SetRoles([]string{"all"}).
		Exec(ctx)
}

func (i *DashInitialize) Identifier() string {
	return "initialize"
}

func (i *DashInitialize) Start(ctx context.Context) error {
	key := "did_init"
	return dao.WithTxEx(ctx, i.db.Client, func(ctx context.Context, client *ent.Client) error {
		exist, err := client.KeyValueStore.Query().Where(keyvaluestore.KeyEQ(key)).Exist(ctx)
		if err != nil {
			return err
		}
		if exist {
			return nil
		}
		_, err = client.KeyValueStore.Create().
			SetKey(key).
			SetValue([]byte(strconv.Itoa(int(time.Now().Unix())))).
			Save(ctx)
		if err != nil {
			return err
		}
		_ = initAdminIfNeed(ctx, client)
		return nil
	})
}

func (i *DashInitialize) Stop(ctx context.Context) error {
	return nil
}
