package telegram

import (
	"context"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tbxark/sphere/pkg/log"
	"golang.org/x/sync/singleflight"
	"strconv"
	"strings"
	"time"
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

func NewGroupMessageFilterMiddleware(trimMention bool, infoExpire time.Duration) bot.Middleware {

	var (
		ts   time.Time
		sf   singleflight.Group
		user *models.User
	)

	isGroupChatType := func(t models.ChatType) bool {
		return t == models.ChatTypeGroup || t == models.ChatTypeSupergroup || t == models.ChatTypeChannel
	}

	getBotInfo := func(ctx context.Context, b *bot.Bot, sf *singleflight.Group) (int64, string, error) {
		v, err, _ := sf.Do("getMe", func() (interface{}, error) {
			// 判断缓存存在且未过期，则直接使用
			if user != nil && time.Since(ts) < infoExpire {
				return user, nil
			}
			// 获取bot信息
			u, err := b.GetMe(ctx)
			if err != nil {
				return nil, err
			}
			user = u
			ts = time.Now()
			return u, nil
		})
		if err != nil {
			return 0, "", err
		}
		return v.(*models.User).ID, v.(*models.User).Username, nil
	}

	checkMention := func(text string, entities []models.MessageEntity, id int64, username string, trimMention bool) (string, bool) {
		isMention := false
		for _, entity := range entities {
			entityStr := text[entity.Offset : entity.Offset+entity.Length]
			switch entity.Type {
			case models.MessageEntityTypeMention: // "mention"适用于有用户名的普通用户
				if entityStr == "@"+username {
					isMention = true
					if trimMention {
						text = text[:entity.Offset] + text[entity.Offset+entity.Length:]
					}
				}
			case models.MessageEntityTypeTextMention: // "text_mention"适用于没有用户名的用户或需要通过ID提及用户的情况
				if entity.User.ID == id {
					isMention = true
					if trimMention {
						text = text[:entity.Offset] + text[entity.Offset+entity.Length:]
					}
				}
			case models.MessageEntityTypeBotCommand: // "bot_command"适用于命令
				if strings.HasSuffix(entityStr, "@"+username) {
					isMention = true
					if trimMention {
						entityStr = strings.ReplaceAll(entityStr, "@"+username, "")
						text = text[:entity.Offset] + entityStr + text[entity.Offset+entity.Length:]
					}
				}
			default:
				continue
			}
		}
		return text, isMention
	}

	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			// 判断是不是群消息，则直接处理
			if update.Message == nil || !isGroupChatType(update.Message.Chat.Type) {
				next(ctx, b, update)
				return
			}

			id, username, err := getBotInfo(ctx, b, &sf)
			if err != nil {
				// 获取bot信息失败，放弃处理
				log.Errorf("get bot info error: %v", err)
				return
			}

			// 判断是不是回复消息，判断回复的消息是否是指定的bot，是则处理
			if update.Message.ReplyToMessage != nil && update.Message.ReplyToMessage.From.ID == id {
				next(ctx, b, update)
				return
			}

			isMention := false

			// 判断Text中是否有提及bot，有则处理
			if update.Message.Entities != nil && update.Message.Text != "" {
				text, mention := checkMention(update.Message.Text, update.Message.Entities, id, username, trimMention)
				update.Message.Text = text
				isMention = mention || isMention
			}

			// 判断Caption中是否有提及bot，有则处理
			if !isMention && update.Message.CaptionEntities != nil && update.Message.Caption != "" {
				text, mention := checkMention(update.Message.Caption, update.Message.CaptionEntities, id, username, trimMention)
				update.Message.Text = text
				isMention = mention || isMention
			}

			// 判断是不是提及了bot，是则处理
			if isMention {
				next(ctx, b, update)
				return
			}
		}
	}
}
