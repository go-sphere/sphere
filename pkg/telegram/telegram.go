package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	bot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/tbxark/go-base-api/pkg/log"
	"github.com/tbxark/go-base-api/pkg/telegram/chat"
	"golang.org/x/sync/singleflight"
)

const HTMLNewLine = "<pre>\n</pre>"

type Telegram[M any] struct {
	bot                       *API
	sf                        singleflight.Group
	member                    map[int64]*M
	commandHandler            map[string]func(*bot.Message) error
	callbackQueryHandler      map[string]func(*bot.CallbackQuery) error
	SuccessfulPaymentHandler  func(*bot.SuccessfulPayment) error
	PreCheckoutQueryHandler   func(*bot.PreCheckoutQuery) error
	DefaultChatMessageHandler func(*bot.Message) error
	DefaultCallbackHandler    func(*bot.CallbackQuery) error
	CreateUserIfNotExist      func(*bot.User) (M, error)
}

func NewTelegram[M any](api *API) *Telegram[M] {
	t := &Telegram[M]{
		bot:                  api,
		sf:                   singleflight.Group{},
		member:               make(map[int64]*M),
		commandHandler:       make(map[string]func(*bot.Message) error),
		callbackQueryHandler: make(map[string]func(*bot.CallbackQuery) error),
	}

	return t
}

func (t *Telegram[M]) Run() {
	go t.bot.InitWebhook()
	t.readUpdate()
}

func (t *Telegram[M]) readUpdate() {
	for update := range t.bot.ReadMessage() {
		u := update
		go func() {
			defer func() {
				if err := recover(); err != nil {
					log.Errorf("<telegram> handle message error: %v", err)
				}
			}()
			t.handleUpdate(u)
		}()
	}
}

func (t *Telegram[M]) GetUser(u *bot.User) (*M, error) {
	if _, exist := t.member[u.ID]; !exist {
		member, err, _ := t.sf.Do(fmt.Sprintf("create:telegram:%d", u.ID), func() (any, error) {
			return t.CreateUserIfNotExist(u)
		})
		if err != nil {
			return nil, err
		}
		m, ok := member.(M)
		if !ok {
			return nil, errors.New("invalid user type")
		}
		return &m, nil
	}
	return t.member[u.ID], nil
}

func (t *Telegram[M]) handleUpdate(update bot.Update) {
	if update.PreCheckoutQuery != nil {
		if t.PreCheckoutQueryHandler != nil {
			_ = t.PreCheckoutQueryHandler(update.PreCheckoutQuery)
		}
	} else if update.Message != nil {
		t.handleMessage(update.Message)
	} else if update.CallbackQuery != nil {
		t.handleCallback(update.CallbackQuery)
	}
}

func (t *Telegram[M]) handleMessage(message *bot.Message) {
	errHandler := func(err error) {
		if err == nil {
			return
		}
		log.Errorf("<Telegram> handle callback error: %v", err)
		var tErr *bot.Error
		if errors.As(err, &tErr) {
		} else {
			_, _ = t.bot.Send(bot.NewMessage(message.Chat.ID, err.Error()))
		}
	}

	// Payment handler
	if t.SuccessfulPaymentHandler != nil {
		errHandler(t.SuccessfulPaymentHandler(message.SuccessfulPayment))
		return
	}

	// Command handler
	if handler, ok := t.commandHandler[message.Command()]; ok {
		errHandler(handler(message))
		return
	}

	// Default handler
	if t.DefaultChatMessageHandler != nil {
		errHandler(t.DefaultChatMessageHandler(message))
	}
}

func (t *Telegram[M]) handleCallback(callback *bot.CallbackQuery) {
	if _, err := t.GetUser(callback.From); err != nil {
		log.Errorf("<telegram> create member error: %v", err)
		return
	}
	callbackRPC := struct {
		Type string `json:"t"`
	}{}
	errorHandler := func(err error) {
		if err == nil {
			return
		}
		log.Errorf("<Telegram> handle callback error: %v", err)
		var tErr *bot.Error
		if !errors.As(err, &tErr) {
			_, _ = t.bot.Send(bot.NewCallback(callback.ID, err.Error()))
		}
	}
	err := json.Unmarshal([]byte(callback.Data), &callbackRPC)
	if err != nil {
		errorHandler(err)
		return
	}
	if handler, ok := t.callbackQueryHandler[callbackRPC.Type]; ok {
		errorHandler(handler(callback))
	} else {
		if t.DefaultCallbackHandler != nil {
			errorHandler(t.DefaultCallbackHandler(callback))
		}
	}
}

func (t *Telegram[M]) CallbackQueryWrapper(handler func(user *M, data string) (*chat.Message, error)) func(callback *bot.CallbackQuery) error {
	return func(callback *bot.CallbackQuery) error {
		id, err := t.GetUser(callback.From)
		if err != nil {
			return err
		}
		msg, err := handler(id, callback.Data)
		if err != nil {
			return err
		}
		return t.EditReplyMessage(callback.Message.Chat.ID, callback.Message.MessageID, msg)
	}
}

func (t *Telegram[M]) BindCallbackQueryHandler(action string, handler func(callback *bot.CallbackQuery) error) {
	t.callbackQueryHandler[action] = handler
}

func (t *Telegram[M]) MessageWrapper(handler func(user *M, data string) (*chat.Message, error)) func(message *bot.Message) error {
	return func(message *bot.Message) error {
		id, err := t.GetUser(message.From)
		if err != nil {
			return err
		}
		msg, err := handler(id, message.Text)
		if err != nil {
			return err
		}
		return t.SendMessage(message.Chat.ID, msg)
	}
}

func (t *Telegram[M]) BindMessageHandler(action string, handler func(message *bot.Message) error) {
	t.commandHandler[action] = handler
}

func (t *Telegram[M]) EditReplyMessage(num int64, messageID int, message *chat.Message) error {
	msg := bot.NewEditMessageText(num, messageID, message.Text)
	if len(message.Sections) > 0 {
		msg.ReplyMarkup = ConvertInlineKeyboardButton(message.Sections)
	}
	_, err := t.bot.Send(msg)
	return err
}

func (t *Telegram[M]) SendMessage(num int64, message *chat.Message) error {
	msg := bot.NewMessage(num, message.Text)
	if len(message.Sections) > 0 {
		msg.ReplyMarkup = ConvertInlineKeyboardButton(message.Sections)
	}
	_, err := t.bot.Send(msg)
	return err
}

func (t *Telegram[M]) SendRequest(req bot.Chattable) error {
	_, err := t.bot.Request(req)
	return err
}

func (t *Telegram[M]) TrimMemberCache(deletable func(k int64, v M) bool) {
	for k, v := range t.member {
		if deletable(k, *v) {
			delete(t.member, k)
		}
	}
}

func ConvertInlineKeyboardButton(sections [][]chat.RPCButton) *bot.InlineKeyboardMarkup {
	buttons := make([][]bot.InlineKeyboardButton, 0)
	for _, section := range sections {
		row := make([]bot.InlineKeyboardButton, 0)
		for _, btn := range section {
			row = append(row, bot.NewInlineKeyboardButtonData(btn.Text, btn.Data))
		}
		buttons = append(buttons, row)
	}
	return &bot.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}
