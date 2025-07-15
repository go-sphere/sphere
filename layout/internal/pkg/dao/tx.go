package dao

import (
	"context"
	"database/sql"

	"github.com/TBXark/sphere/layout/internal/pkg/database/ent"
	"github.com/TBXark/sphere/log"
	"github.com/TBXark/sphere/log/logfields"
)

type txOptions struct {
	Isolation    sql.IsolationLevel
	ReadOnly     bool
	CommitHook   ent.CommitHook
	RollbackHook ent.RollbackHook
}

type Option func(*txOptions)

func WithTxIsolation(level sql.IsolationLevel) Option {
	return func(opts *txOptions) {
		opts.Isolation = level
	}
}

func WithTxReadOnly(readOnly bool) Option {
	return func(opts *txOptions) {
		opts.ReadOnly = readOnly
	}
}

func WithTxCommitHook(hook ent.CommitHook) Option {
	return func(opts *txOptions) {
		opts.CommitHook = hook
	}
}

func WithTxRollbackHook(hook ent.RollbackHook) Option {
	return func(opts *txOptions) {
		opts.RollbackHook = hook
	}
}

func newTxOptions(opts ...Option) *txOptions {
	txOpts := &txOptions{
		Isolation:    sql.LevelDefault,
		ReadOnly:     false,
		CommitHook:   nil,
		RollbackHook: nil,
	}
	for _, opt := range opts {
		opt(txOpts)
	}
	return txOpts
}

func WithTx[T any](ctx context.Context, db *ent.Client, exe func(ctx context.Context, tx *ent.Client) (*T, error), opts ...Option) (*T, error) {
	txOpts := newTxOptions(opts...)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		Isolation: txOpts.Isolation,
		ReadOnly:  txOpts.ReadOnly,
	})
	if err != nil {
		return nil, err
	}
	if txOpts.CommitHook != nil {
		tx.OnCommit(txOpts.CommitHook)
	}
	if txOpts.RollbackHook != nil {
		tx.OnRollback(txOpts.RollbackHook)
	}
	defer func() {
		if reason := recover(); reason != nil {
			log.Warnw("WithTx panic", logfields.Any("error", reason))
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

func WithTxEx(ctx context.Context, db *ent.Client, exe func(ctx context.Context, tx *ent.Client) error, opts ...Option) error {
	txOpts := newTxOptions(opts...)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		Isolation: txOpts.Isolation,
		ReadOnly:  txOpts.ReadOnly,
	})
	if err != nil {
		return err
	}
	if txOpts.CommitHook != nil {
		tx.OnCommit(txOpts.CommitHook)
	}
	if txOpts.RollbackHook != nil {
		tx.OnRollback(txOpts.RollbackHook)
	}
	defer func() {
		if reason := recover(); reason != nil {
			log.Warnw("WithTxEx panic", logfields.Any("error", reason))
			_ = tx.Rollback()
			return
		}
	}()
	err = exe(ctx, tx.Client())
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
