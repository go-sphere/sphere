package dao

import (
	"context"
	"encoding/json"
	keyvaluestore2 "github.com/TBXark/sphere/internal/pkg/database/ent/keyvaluestore"
)

func GetKeyValueStore[T any](ctx context.Context, dao *Dao, key string) (*T, error) {
	value, err := dao.KeyValueStore.Query().Where(keyvaluestore2.KeyEQ(key)).Only(ctx)
	if err != nil {
		return nil, err
	}
	var res T
	err = json.Unmarshal(value.Value, &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func SetSystemConfig[T any](ctx context.Context, dao *Dao, key string, value *T) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	err = dao.KeyValueStore.Create().
		SetKey(key).
		SetValue(data).
		OnConflictColumns(keyvaluestore2.FieldKey).
		SetValue(data).
		Exec(ctx)
	return err
}

type SystemConfig struct {
	ExampleField string `json:"example_field"`
}

const SystemConfigKey = "system_config"

func (d *Dao) GetSystemConfig(ctx context.Context) (*SystemConfig, error) {
	return GetKeyValueStore[SystemConfig](ctx, d, SystemConfigKey)
}

func (d *Dao) SetSystemConfig(ctx context.Context, config *SystemConfig) error {
	return SetSystemConfig(ctx, d, SystemConfigKey, config)
}
