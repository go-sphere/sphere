package dao

import (
	"context"
	_ "github.com/go-sql-driver/mysql"
	"github.com/tbxark/go-base-api/pkg/dao/ent"
	"github.com/tbxark/go-base-api/pkg/log"
	"github.com/tbxark/go-base-api/pkg/log/field"
)

type Config struct {
	Type  string `json:"type"`
	Path  string `json:"path"`
	Debug bool   `json:"debug"`
}

type Database struct {
	*ent.Client
}

func NewDatabase(config *Config) (*Database, error) {
	client, err := ent.Open(config.Type, config.Path)
	if err != nil {
		return nil, err
	}
	if e := client.Schema.Create(context.Background()); e != nil {
		return nil, e
	}
	if config.Debug {
		client = client.Debug()
	}
	return &Database{client}, nil
}

func WithTx[T any](ctx context.Context, db *Database, exe func(ctx context.Context, tx *Database) (*T, error)) (*T, error) {
	tx, tErr := db.BeginTx(ctx, nil)
	if tErr != nil {
		return nil, tErr
	}
	defer func() {
		if e := recover(); e != nil {
			log.Warnw(
				"WithTx panic",
				field.Any("error", e),
			)
			_ = tx.Rollback()
			return
		}
	}()
	result, err := exe(ctx, &Database{tx.Client()})
	if err != nil {
		if rErr := tx.Rollback(); rErr != nil {
			return nil, rErr
		}
		return nil, err
	}
	if cErr := tx.Commit(); cErr != nil {
		return nil, cErr
	}
	return result, err
}

func WithTxEx(ctx context.Context, db *Database, exe func(ctx context.Context, tx *Database) error) error {
	tx, tErr := db.BeginTx(ctx, nil)
	if tErr != nil {
		return tErr
	}
	defer func() {
		if e := recover(); e != nil {
			log.Warnw(
				"WithTxEx panic",
				field.Any("error", e),
			)
			_ = tx.Rollback()
			return
		}
	}()
	err := exe(ctx, &Database{tx.Client()})
	if err != nil {
		if rErr := tx.Rollback(); rErr != nil {
			return rErr
		}
		return err
	}
	if cErr := tx.Commit(); cErr != nil {
		return cErr
	}
	return err
}
