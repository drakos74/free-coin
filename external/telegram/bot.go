package telegram

import (
	"context"
	"fmt"
	"time"

	"github.com/drakos74/free-coin/coinapi"
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

// consumerKey is the internal consumer key for indexing and managing consumers.
type consumerKey struct {
	key    string
	prefix string
}

// Bot defines the telegram bot coinapi.UserInterface api implementation.
type Bot struct {
	bot             botAPI
	messages        map[int]string
	triggers        map[string]*coinapi.Trigger
	blockedTriggers map[string]time.Time
	consumers       map[consumerKey]chan coinapi.Command
}

// NewBot creates a new telegram bot implementing the coinapi.UserInterface api.
func NewBot() (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(BotToken)
	if err != nil {
		return nil, fmt.Errorf("error creating bot: %w", err)
	}
	//bot.Debug = true
	bot.Buffer = 0
	return &Bot{
		bot:             bot,
		messages:        make(map[int]string),
		triggers:        make(map[string]*coinapi.Trigger),
		blockedTriggers: make(map[string]time.Time),
		consumers:       make(map[consumerKey]chan coinapi.Command),
	}, nil
}

// Run starts the Bot and polls for updates from telegram.
func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	//u.Timeout = 60

	updates, err := b.bot.GetUpdatesChan(u)
	if err != nil {
		return err
	}

	go b.listenToUpdates(ctx, updates)
	return nil
}

// Listen exposes a channel to the caller with updates for the given prefix.
func (b *Bot) Listen(key, prefix string) <-chan coinapi.Command {
	ch := make(chan coinapi.Command)
	b.consumers[consumerKey{
		key:    key,
		prefix: prefix,
	}] = ch
	return ch
}

// Send sends the given message with the attached details to the specified telegram chat.
func (b *Bot) Send(message *coinapi.Message, trigger *coinapi.Trigger) int {
	msg := NewMessage(message)
	msgID, err := b.send(msg, trigger)
	if err != nil {
		log.Err(err).Msg("could not send message")
		return 0
	}
	return msgID
}

// send will send a message and store the appropriate trigger.
// it will automatically execute the default command if user does not reply.
// TODO : send confirmation of auto-invoke - use tgbotapi.Message here
func (b *Bot) send(msg tgbotapi.MessageConfig, trigger *coinapi.Trigger) (int, error) {
	// before sending check for blocked triggers ...
	if txt, ok := b.checkIfBlocked(trigger); ok {
		sent, err := b.bot.Send(addLine(msg, txt))
		return sent.MessageID, err
	}
	// otherwise send the message and add the trigger
	// TODO : be able to expose more details on the trigger
	if trigger != nil {
		msg = addLine(msg, fmt.Sprintf("[trigger] %vm", trigger.Timeout.Minutes()))
	}
	sent, err := b.bot.Send(msg)
	if err != nil {
		return 0, err
	}
	if trigger != nil {
		// store the message for potential replies on the trigger.
		b.messages[sent.MessageID] = trigger.ID
		b.triggers[trigger.ID] = trigger
		t := defaultTimeout
		if trigger.Timeout > 0 {
			t = trigger.Timeout
		}
		if len(trigger.Default) > 0 {
			go b.deferExecute(t, sent.MessageID, trigger)
		}
	}
	return sent.MessageID, nil
}

// checkIfBlocked checks if the trigger is currently blocked.
func (b *Bot) checkIfBlocked(trigger *coinapi.Trigger) (string, bool) {
	if trigger != nil {
		if blockedTime, ok := b.blockedTriggers[trigger.ID]; ok {
			// trigger has been blocked
			blocked := time.Now().Sub(blockedTime)
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
