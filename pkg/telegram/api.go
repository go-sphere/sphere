package telegram

import (
	"encoding/json"
	bot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/tbxark/go-base-api/pkg/log"
	"io"
	"net/http"
)

type APIConfig struct {
	Token   string `json:"token"`
	Webhook string `json:"webhook"`
}

type API struct {
	*bot.BotAPI
	webhook *bot.WebhookConfig
	msgChan chan bot.Update
}

func NewAPI(config *APIConfig) *API {
	api, err := bot.NewBotAPI(config.Token)
	if err != nil {
		log.Panic(err)
	}
	log.Debugf("Authorized on account %s", api.Self.UserName)
	t := &API{
		BotAPI:  api,
		webhook: nil,
		msgChan: make(chan bot.Update, 100),
	}

	if config.Webhook != "" {
		if c, e := bot.NewWebhook(config.Webhook); e == nil {
			t.webhook = &c
		}
	}

	return t
}

func (t *API) SendMessage(UID int64, message string) error {
	msg := bot.NewMessage(UID, message)
	msg.ParseMode = "Markdown"
	_, err := t.Send(msg)
	if err != nil {
		msg.ParseMode = ""
		_, err = t.Send(msg)
		return err
	}
	return nil
}

func (t *API) InitWebhook() {
	if t.webhook == nil {
		_, _ = t.Send(bot.DeleteWebhookConfig{})
	} else {
		_, _ = t.Send(*t.webhook)
	}
}

func (t *API) ReadMessage() <-chan bot.Update {
	if t.webhook != nil {
		return t.msgChan
	} else {
		u := bot.NewUpdate(0)
		u.Timeout = 60
		return t.GetUpdatesChan(u)
	}
}

func (t *API) HandleWebhook(token string, req *http.Request) {
	if t.Token != token {
		return
	}
	var update bot.Update
	bytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.Debugf("read webhook body error: %v", err)
		return
	}
	err = json.Unmarshal(bytes, &update)
	if err != nil {
		log.Debugf("unmarshal webhook body error: %v", err)
		return
	}
	t.msgChan <- update
}
