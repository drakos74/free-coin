package telegram

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/drakos74/free-coin/internal/api"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/rs/zerolog/log"
)

const (
	telegramBotToken = "TELEGRAM_BOT_TOKEN"
	telegramChatID   = "TELEGRAM_CHAT_ID"
)

// allow to change these for the tests
var (
	// defaultTimeout defines the trigger execution timeout, if none is provided in the trigger.
	defaultTimeout = 30 * time.Second
	// blockTimeout defines the duration that a trigger is blocked if a reply comes up.
	blockTimeout = 30 * time.Second
)

type botAPI interface {
	GetUpdatesChan(config tgbotapi.UpdateConfig) (tgbotapi.UpdatesChannel, error)
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

// consumerKey is the internal consumer key for indexing and managing consumers.
type consumerKey struct {
	key    string
	prefix string
}

type executableTrigger struct {
	message *tgbotapi.Message
	trigger *api.Trigger
	replyID int
}

// Bot defines the telegram bot coinapi.UserInterface api implementation.
type Bot struct {
	bot             botAPI
	process         chan executableTrigger
	messages        map[int]string
	triggers        map[string]*api.Trigger
	blockedTriggers map[string]time.Time
	consumers       map[consumerKey]chan api.Command
}

// NewBot creates a new telegram bot implementing the coinapi.UserInterface api.
func NewBot() (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(os.Getenv(telegramBotToken))
	if err != nil {
		return nil, fmt.Errorf("error creating bot: %w", err)
	}
	//bot.Debug = true
	bot.Buffer = 0
	return &Bot{
		bot:             bot,
		process:         make(chan executableTrigger),
		messages:        make(map[int]string),
		triggers:        make(map[string]*api.Trigger),
		blockedTriggers: make(map[string]time.Time),
		consumers:       make(map[consumerKey]chan api.Command),
	}, nil
}

// Run starts the Bot and polls for updates from telegram.
func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 10

	updates, err := b.bot.GetUpdatesChan(u)
	if err != nil {
		return err
	}

	go b.processExecution()
	go b.listenToUpdates(ctx, updates)
	return nil
}

// Listen exposes a channel to the caller with updates for the given prefix.
func (b *Bot) Listen(key, prefix string) <-chan api.Command {
	ch := make(chan api.Command)
	b.consumers[consumerKey{
		key:    key,
		prefix: prefix,
	}] = ch
	return ch
}

// Send sends the given message with the attached details to the specified telegram chat.
func (b *Bot) Send(message *api.Message, trigger *api.Trigger) int {
	msg := NewMessage(message)
	msgID, err := b.send(msg, trigger)
	if err != nil {
		log.Err(err).Msg("could not send message")
		return 0
	}
	return msgID
}

// checkIfBlocked checks if the trigger is currently blocked.
func (b *Bot) checkIfBlocked(trigger *api.Trigger) (string, bool) {
	if trigger != nil {
		if blockedTime, ok := b.blockedTriggers[trigger.ID]; ok {
			// trigger has been blocked
			blocked := time.Since(blockedTime)
			if blocked > blockTimeout {
				// unblock ...
				delete(b.blockedTriggers, trigger.ID)
				return "", false
			} else {
				// trigger is blocked return at this point.
				return fmt.Sprintf("[auto-trigger blocked for %.0fs]", blockTimeout.Seconds()-blocked.Seconds()), true
			}
		}
	}
	return "", false
}
