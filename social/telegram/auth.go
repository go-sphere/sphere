package telegram

import (
	"context"

	"github.com/go-sphere/sphere/utils/contextutil/metadata"
	"github.com/go-telegram/bot/models"
)

// AuthExtractor defines the interface for extracting authentication information from Telegram updates.
// Implementations should extract user identity and authorization data from the update context.
type AuthExtractor interface {
	ExtractorAuth(ctx context.Context, update *Update) (map[string]any, error)
}

// AuthExtractorFunc is a function type that implements the AuthExtractor interface.
// It allows using functions as AuthExtractor implementations.
type AuthExtractorFunc func(ctx context.Context, update *Update) (map[string]any, error)

// ExtractorAuth implements the AuthExtractor interface by calling the function.
func (f AuthExtractorFunc) ExtractorAuth(ctx context.Context, update *Update) (map[string]any, error) {
	return f(ctx, update)
}

// NewAuthMiddleware creates a middleware that extracts authentication information from updates.
// It uses the provided AuthExtractor to get user data and injects it into the request context
// using metadata. The extracted data becomes available to downstream handlers.
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

// DefaultAuthExtractor is the default implementation for extracting authentication data from updates.
// It extracts user ID and username from either message or callback query updates.
// Returns a map containing "uid" (user ID) and "subject" (username) if a user is found.
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
