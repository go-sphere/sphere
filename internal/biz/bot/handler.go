package bot

import (
	"context"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tbxark/go-base-api/pkg/log"
	"golang.org/x/sync/singleflight"
	"strconv"
	"strings"
)

type HandlerFunc func(ctx context.Context, update *models.Update) error

type HandlerMiddleware func(next HandlerFunc) HandlerFunc

func (h HandlerFunc) WithMiddleware(middleware ...HandlerMiddleware) HandlerFunc {
	handler := h
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}

func NewSingleFlightMiddleware() HandlerMiddleware {
	sf := &singleflight.Group{}
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, update *models.Update) error {
			if update.CallbackQuery == nil {
				return next(ctx, update)
			}
			key := strconv.Itoa(update.CallbackQuery.Message.Message.ID)
			_, err, _ := sf.Do(key, func() (interface{}, error) {
				return nil, next(ctx, update)
			})
			return err
		}
	}
}

func NewErrorAlertMiddleware(b *bot.Bot) HandlerMiddleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, update *models.Update) error {
			err := next(ctx, update)
			if err != nil {
				if update.Message != nil {
					_, e := b.SendMessage(ctx, &bot.SendMessageParams{
						ChatID: update.Message.Chat.ID,
						Text:   err.Error(),
					})
					return e
				}
				if update.CallbackQuery != nil {
					_, e := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
						CallbackQueryID: update.CallbackQuery.ID,
						Text:            err.Error(),
					})
					return e
				}
			}
			return nil
		}
	}
}

func NewRecoveryMiddleware() bot.Middleware {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("bot panic: %v", r)
				}
			}()
			next(ctx, bot, update)
		}
	}
}

func NewGroupMessageFilterMiddleware(id int64, username string, trimMention bool) bot.Middleware {
	isGroupChatType := func(t string) bool {
		switch t {
		case "group", "supergroup":
			return true
		default:
			return false
		}
	}
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, bot *bot.Bot, update *models.Update) {
			// 判断是不是群消息，则直接处理
			if update.Message == nil || isGroupChatType(update.Message.Chat.Type) {
				next(ctx, bot, update)
				return
			}
			// 判断是不是回复消息，判断回复的消息是否是指定的bot，是则处理
			if update.Message.ReplyToMessage != nil && update.Message.ReplyToMessage.From.ID == id {
				next(ctx, bot, update)
				return
			}
			// 判断是不是直接提到了bot，是则处理
			if update.Message.Entities != nil {
				isMention := false
				for _, entity := range update.Message.Entities {
					switch entity.Type {
					case "mention", "text_mention": // mention 和 text_mention 是用户提到的实体
						if entity.User.ID == id {
							isMention = true
							if trimMention {
								update.Message.Text = update.Message.Text[:entity.Offset] + update.Message.Text[entity.Offset+entity.Length:]
							}
						}
						break
					case "bot_command": // bot_command 是命令实体
						entityStr := update.Message.Text[entity.Offset : entity.Offset+entity.Length]
						if strings.HasSuffix(entityStr, "@"+username) {
							isMention = true
							if trimMention {
								entityStr = strings.ReplaceAll(entityStr, "@"+entity.User.Username, "")
								update.Message.Text = update.Message.Text[:entity.Offset] + entityStr + update.Message.Text[entity.Offset+entity.Length:]
							}
						}
						break
					default:
						continue
					}
				}
				if isMention {
					next(ctx, bot, update)
				}
			}
		}
	}
}
