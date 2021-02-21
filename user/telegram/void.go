package telegram

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type Void struct {
}

func (v Void) GetUpdatesChan(config tgbotapi.UpdateConfig) (tgbotapi.UpdatesChannel, error) {
	return make(chan tgbotapi.Update), nil
}

func (v Void) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	return tgbotapi.Message{}, fmt.Errorf("could not send message")
}
