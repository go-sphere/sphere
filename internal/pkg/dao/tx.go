package dao

import (
	"context"
	"github.com/TBXark/sphere/internal/pkg/database/ent"
	"github.com/TBXark/sphere/pkg/log"
	"github.com/TBXark/sphere/pkg/log/logfields"
)

func WithTx[T any](ctx context.Context, db *ent.Client, exe func(ctx context.Context, tx *ent.Client) (*T, error)) (*T, error) {
	tx, tErr := db.BeginTx(ctx, nil)
	if tErr != nil {
		return nil, tErr
	}
	defer func() {
		if e := recover(); e != nil {
			log.Warnw(
				"WithTx panic",
				logfields.Any("error", e),
			)
			_ = tx.Rollback()
			return
		}
	}()
	result, err := exe(ctx, tx.Client())
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

func WithTxEx(ctx context.Context, db *ent.Client, exe func(ctx context.Context, tx *ent.Client) error) error {
	tx, tErr := db.BeginTx(ctx, nil)
	if tErr != nil {
		return tErr
	}
	defer func() {
		if e := recover(); e != nil {
			log.Warnw(
				"WithTxEx panic",
				logfields.Any("error", e),
			)
			_ = tx.Rollback()
			return
		}
	}()
	err := exe(ctx, tx.Client())
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
