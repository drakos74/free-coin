package telegram

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/rs/zerolog/log"
)

const (
	telegramBotToken        = "TELEGRAM_BOT_TOKEN"
	telegramChatID          = "TELEGRAM_CHAT_ID"
	privateTelegramBotToken = "PRIVATE_TELEGRAM_BOT_TOKEN"
	privateTelegramChatID   = "PRIVATE_TELEGRAM_CHAT_ID"
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

type executableTrigger struct {
	message *tgbotapi.Message
	replyID int
	private api.Index
}

// Bot defines the telegram bot coinapi.User api implementation.
type Bot struct {
	publicBot       botAPI
	publicChatID    int64
	privateBot      botAPI
	privateChatID   int64
	messages        map[int]string
	triggers        map[string]*api.Trigger
	blockedTriggers map[string]time.Time
	consumers       map[api.ConsumerKey]chan api.Command
	lock            *sync.Mutex
}

// NewBot creates a new telegram bot implementing the coinapi.User api.
func NewBot() (*Bot, error) {
	// public bot set up
	publicBot, err := tgbotapi.NewBotAPI(os.Getenv(telegramBotToken))
	if err != nil {
		return nil, fmt.Errorf("error creating bot: %w", err)
	}
	publicChatIDProperty := os.Getenv(telegramChatID)
	publicChatID, err := strconv.ParseInt(publicChatIDProperty, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing public chat ID: %w", err)
	}
	// private bot set up
	privateBot, err := tgbotapi.NewBotAPI(os.Getenv(privateTelegramBotToken))
	if err != nil {
		return nil, fmt.Errorf("error creating bot: %w", err)
	}
	privateChatIDProperty := os.Getenv(privateTelegramChatID)
	privateChatID, err := strconv.ParseInt(privateChatIDProperty, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing public chat ID: %w", err)
	}
	//bot.Debug = true
	publicBot.Buffer = 0
	privateBot.Buffer = 0
	return &Bot{
		publicBot:       publicBot,
		publicChatID:    publicChatID,
		privateBot:      privateBot,
		privateChatID:   privateChatID,
		messages:        make(map[int]string),
		triggers:        make(map[string]*api.Trigger),
		blockedTriggers: make(map[string]time.Time),
		consumers:       make(map[api.ConsumerKey]chan api.Command),
		lock:            new(sync.Mutex),
	}, nil
}

// Run starts the Bot and polls for updates from telegram.
func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 10

	publicUpdates, err := b.publicBot.GetUpdatesChan(u)
	if err != nil {
		return err
	}

	privateUpdates, err := b.privateBot.GetUpdatesChan(u)
	if err != nil {
		return err
	}

	go b.listenToUpdates(ctx, api.Public, publicUpdates)
	go b.listenToUpdates(ctx, api.Private, privateUpdates)
	return nil
}

// Listen exposes a channel to the caller with updates for the given prefix.
func (b *Bot) Listen(key, prefix string) <-chan api.Command {
	b.lock.Lock()
	defer b.lock.Unlock()
	ch := make(chan api.Command)
	b.consumers[api.ConsumerKey{
		Key:    key,
		Prefix: prefix,
	}] = ch
	return ch
}

// Send sends the given message with the attached details to the specified telegram chat.
func (b *Bot) Send(private api.Index, message *api.Message, trigger *api.Trigger) int {
	msg := b.newMessage(private, message)
	msgID, err := b.send(private, msg, trigger)
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
