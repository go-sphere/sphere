package dao

import (
	"context"
	"encoding/json"
	"github.com/tbxark/go-base-api/pkg/dao/ent/keyvaluestore"
)

type SystemConfig struct {
	ExampleField string `json:"example_field"`
}

const SystemConfigKey = "system_config"

func GetSystemConfig(ctx context.Context, db *Database) (*SystemConfig, error) {
	var config SystemConfig
	value, err := db.KeyValueStore.Query().Where(keyvaluestore.KeyEQ(SystemConfigKey)).Only(ctx)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(value.Value, &config)
	return &config, nil
}

func SetSystemConfig(ctx context.Context, db *Database, config *SystemConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	err = db.KeyValueStore.Create().SetKey(SystemConfigKey).
		SetValue(data).
		OnConflictColumns(keyvaluestore.FieldKey).
		SetValue(data).
		Exec(ctx)
	return err
}
