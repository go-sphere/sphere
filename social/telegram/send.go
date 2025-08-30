package telegram

import (
	"context"
	"errors"
	"time"

	"github.com/go-telegram/bot"
	"golang.org/x/time/rate"
)

// SendMessage sends or edits a message based on the update type and content.
// For callback queries, it edits the original message. For regular messages, it sends a new message.
// The function automatically chooses between text and photo messages based on media presence.
func SendMessage(ctx context.Context, b *bot.Bot, update *Update, m *Message) error {
	if m == nil || update == nil {
		return nil
	}
	if update.CallbackQuery != nil {
		origin := update.CallbackQuery.Message.Message
		if len(origin.Photo) == 0 {
			param := m.toEditMessageTextParams(origin.Chat.ID, origin.ID)
			_, err := b.EditMessageText(ctx, param)
			return err
		} else {
			if m.Media == nil {
				param := m.toEditMessageCaptionParams(origin.Chat.ID, origin.ID)
				_, err := b.EditMessageCaption(ctx, param)
				return err
			} else {
				param := m.toEditMessageMediaParams(origin.Chat.ID, origin.ID)
				_, err := b.EditMessageMedia(ctx, param)
				return err
			}
		}
	}
	if update.Message != nil {
		if m.Media == nil {
			param := m.toSendMessageParams(update.Message.Chat.ID)
			_, err := b.SendMessage(ctx, param)
			return err
		} else {
			param := m.toSendPhotoParams(update.Message.Chat.ID)
			_, err := b.SendPhoto(ctx, param)
			return err
		}
	}
	return nil
}

// SendErrorMessage sends an error message to the user based on the update type.
// For regular messages, it sends a new message with the error text.
// For callback queries, it shows the error in a popup using AnswerCallbackQuery.
func SendErrorMessage(ctx context.Context, b *bot.Bot, update *Update, err error) {
	if err == nil {
		return
	}
	if update.Message != nil {
		_, _ = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   err.Error(),
		})
	}
	if update.CallbackQuery != nil {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: update.CallbackQuery.ID,
			Text:            err.Error(),
		})
	}
}

// broadcastOptions holds configuration for message broadcasting operations.
type broadcastOptions struct {
	progress            func(int, int, int) // Progress callback: (current, errors, total)
	terminalOnSendError bool                // Whether to stop on first send error
}

// BroadcastOption defines a function type for configuring broadcast operations.
type BroadcastOption func(*broadcastOptions)

func newBroadcastOptions(opts ...BroadcastOption) *broadcastOptions {
	defaults := &broadcastOptions{
		progress:            nil,
		terminalOnSendError: false,
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

// WithProgress sets a progress callback function for broadcast operations.
// The callback receives (current index, error count, total count) during broadcasting.
func WithProgress(progress func(int, int, int)) BroadcastOption {
	return func(o *broadcastOptions) {
		o.progress = progress
	}
}

// WithTerminalOnSendError configures whether broadcasting should stop on the first send error.
// If true, broadcasting terminates immediately on any send failure.
func WithTerminalOnSendError(terminalOnSendError bool) BroadcastOption {
	return func(o *broadcastOptions) {
		o.terminalOnSendError = terminalOnSendError
	}
}

// BroadcastMessage sends messages to multiple recipients with rate limiting and error handling.
// It processes each item in the data slice through the provided send function, respecting
// the rate limiter and reporting progress through optional callbacks.
//
// Type parameter T represents the data type for each broadcast target.
//
// Parameters:
//   - ctx: Context for cancellation
//   - b: Bot instance for sending messages
//   - data: Slice of data items to process
//   - rateLimiter: Rate limiter to control send frequency
//   - send: Function to send message for each data item
//   - options: Optional configuration for progress tracking and error handling
func BroadcastMessage[T any](ctx context.Context, b *bot.Bot, data []T, rateLimiter *rate.Limiter, send func(context.Context, *bot.Bot, T) error, options ...BroadcastOption) error {
	opts := newBroadcastOptions(options...)
	total := len(data)
	errCount := 0
	for i, d := range data {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if opts.progress != nil {
			opts.progress(i, errCount, total)
		}
		err := rateLimiter.Wait(ctx)
		if err != nil {
			return err
		}
		err = send(ctx, b, d)
		if err != nil {
			errCount++
			if opts.terminalOnSendError {
				return err
			}
		}
	}
	if opts.progress != nil {
		opts.progress(total, errCount, total)
	}
	return nil
}

// RetryOnTooManyRequestsError implements automatic retry logic for Telegram rate limit errors.
// It respects the RetryAfter duration from Telegram's error response and retries up to maxRetries times.
// Returns an error if max retries are exceeded or if a non-rate-limit error occurs.
func RetryOnTooManyRequestsError(maxRetries int, send func() error) error {
	if maxRetries < 0 {
		return errors.New("max retries exceeded")
	}
	err := send()
	if err == nil {
		return nil
	}
	var tooManyRequestsError *bot.TooManyRequestsError
	if errors.As(err, &tooManyRequestsError) {
		sleepDuration := time.Duration(tooManyRequestsError.RetryAfter) * time.Second
		time.Sleep(sleepDuration)
		return RetryOnTooManyRequestsError(maxRetries-1, send)
	}
	return err
}
