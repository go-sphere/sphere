package telegram

import (
	"context"

	"github.com/TBXark/sphere/utils/contextutil/metadata"
	"github.com/go-telegram/bot/models"
)

type AuthExtractor interface {
	ExtractorAuth(ctx context.Context, update *Update) (map[string]any, error)
}

type AuthExtractorFunc func(ctx context.Context, update *Update) (map[string]any, error)

func (f AuthExtractorFunc) ExtractorAuth(ctx context.Context, update *Update) (map[string]any, error) {
	return f(ctx, update)
}

func NewAuthMiddleware(auth AuthExtractor) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, update *Update) error {
			info, err := auth.ExtractorAuth(ctx, update)
			if err != nil {
				return err
			}
			return next(metadata.WithValues(ctx, info), update)
		}
	}
}

func DefaultAuthExtractor(ctx context.Context, update *Update) (map[string]any, error) {
	var user *models.User
	if update.Message != nil {
		user = update.Message.From
	}
	if update.CallbackQuery != nil {
		user = &update.CallbackQuery.From
	}
	if user == nil {
		return nil, nil
	}
	return map[string]any{
		"uid":     user.ID,
		"subject": user.Username,
	}, nil
}
