package telegram

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/account"

	"github.com/drakos74/free-coin/internal/api"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/rs/zerolog/log"
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

type bot struct {
	b  botAPI
	id int64
}

func (b bot) String() string {
	return fmt.Sprintf("b = %+v | id = %+v\n", b.b, b.id)
}

// Bot defines the telegram bot coinapi.User api implementation.
type Bot struct {
	chat            map[api.Index]*bot
	messages        map[int]string
	triggers        map[string]*api.Trigger
	blockedTriggers map[string]time.Time
	consumers       map[api.ConsumerKey]chan api.Command
	lock            *sync.Mutex
}

// NewBot creates a new telegram bot implementing the coinapi.User api.
func NewBot(idxs ...api.Index) (*Bot, error) {
	chat := make(map[api.Index]*bot)
	for _, idx := range idxs {
		format := account.NewFormat(account.Name(idx), "")
		// external bot set up
		bt, err := tgbotapi.NewBotAPI(os.Getenv(format.Token()))
		if err != nil {
			return nil, fmt.Errorf("error creating bot: %w", err)
		}
		chatIDProperty := os.Getenv(format.ChatID())
		chatID, err := strconv.ParseInt(chatIDProperty, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing public chat ID: %w", err)
		}
		bt.Buffer = 0
		chat[idx] = &bot{
			b:  bt,
			id: chatID,
		}
		log.Info().Str("index", string(idx)).Msg("bot started")
	}

	fmt.Printf("chat = %+v\n", chat)

	return &Bot{
		chat:            chat,
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

	// listen to updates for the configured indexes
	for idx, bot := range b.chat {
		updates, err := bot.b.GetUpdatesChan(u)
		if err != nil {
			return err
		}
		go b.listenToUpdates(ctx, idx, updates)
	}
	return nil
}

// Listen exposes a channel to the caller with updates for the given prefix.
func (b *Bot) Listen(key, prefix string) <-chan api.Command {
	b.lock.Lock()
	defer b.lock.Unlock()
	log.Info().Str("key", key).Str("prefix", prefix).Msg("registered listener")
	ch := make(chan api.Command)
	b.consumers[api.ConsumerKey{
		Key:    key,
		Prefix: prefix,
	}] = ch
	return ch
}

// Send sends the given message with the attached details to the specified telegram chat.
func (b *Bot) Send(private api.Index, message *api.Message, trigger *api.Trigger) int {
	msg, err := b.newMessage(private, message)
	if err != nil {
		log.Err(err).Msg("could not create message")
		return 0
	}
	msgID, err := b.send(private, msg, trigger)
	if err != nil {
		log.Err(err).Msg("could not send message")
		return 0
	}
	return msgID
}

// AddUser assigns a user to a channel by ID
func (b *Bot) AddUser(channel api.Index, user string, chatID int64) error {
	if bb, ok := b.chat[channel]; ok {
		if chatID == 0 {
			chatID = bb.id
		}
		b.chat[api.Index(user)] = &bot{
			b:  bb.b,
			id: chatID,
		}
		return nil
	}
	return fmt.Errorf("bot does not exist: '%s'", channel)
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
