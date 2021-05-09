package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/drakos74/free-coin/user"

	"github.com/google/uuid"

	"github.com/drakos74/free-coin/internal/api"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/rs/zerolog/log"
)

// NewMessage creates a new telegram message config
func (b *Bot) newMessage(idx api.Index, message *api.Message) (tgbotapi.MessageConfig, error) {
	if bot, ok := b.chat[idx]; ok {
		tgChatID := bot.id
		msg := tgbotapi.NewMessage(tgChatID, message.Text)
		if message.Reply > 0 {
			msg.ReplyToMessageID = message.Reply
		}
		return msg, nil
	}
	return tgbotapi.MessageConfig{}, fmt.Errorf("index not defined: %v", idx)
}

func addLine(msg tgbotapi.MessageConfig, txt string) tgbotapi.MessageConfig {
	msg.Text = fmt.Sprintf("%s\n%s", msg.Text, txt)
	return msg
}

// listenToUpdates listens to updates for the telegram bot.
func (b *Bot) listenToUpdates(ctx context.Context, private api.Index, updates tgbotapi.UpdatesChannel) {
	for {
		select {
		case update := <-updates:
			if update.Message == nil { // ignore any non-Message Updates
				continue
			}

			reply := update.Message.ReplyToMessage
			if reply != nil {
				var chatID int64
				if update.Message.Chat != nil {
					chatID = update.Message.Chat.ID
				}
				log.Info().
					Str("from", update.Message.From.UserName).
					Str("text", update.Message.Text).
					Int64("chat", chatID).
					Str("private", fmt.Sprintf("%v", private)).
					Int("messageID", reply.MessageID).
					Msg("reply received")
				// if it s a reply , we are responsible to act on the trigger
				b.execute(private, update.Message, reply.MessageID)
				continue
			}
			var chatID int64
			if update.Message.Chat != nil {
				chatID = update.Message.Chat.ID
			}

			user := fmt.Sprintf("%+v", update.Message.From)
			log.Debug().
				Str("from", fmt.Sprintf("%+v", update.Message.From)).
				Str("text", update.Message.Text).
				Int64("chat", chatID).
				Str("private", fmt.Sprintf("%v", private)).
				Msg("message received")
			// panic ...
			if update.Message.Text == "?potso" &&
				(user == "Vagz" || user == "moneytized") {
				panic(fmt.Sprintf("panic signal received from '%s' !", user))
			}
			b.lock.Lock()
			for k, consumer := range b.consumers {
				// propagate the message
				if strings.HasPrefix(update.Message.Text, k.Prefix) {
					log.Info().
						Str("from", fmt.Sprintf("%+v", update.Message.From)).
						Str("text", update.Message.Text).
						Str("consumer", fmt.Sprintf("%+v", k)).
						Msg("message propagated")
					select {
					case consumer <- api.Command{
						ID:      update.Message.MessageID,
						User:    fmt.Sprintf("%v", update.Message.From),
						Content: update.Message.Text,
					}:
						// TODO : wait until consumer has processed !
					case <-time.After(1 * time.Second):
						log.Warn().Str("prefix", k.Prefix).Str("consumer", fmt.Sprintf("%+v", k)).Str("command", update.Message.Text).Msg("consumer did not receive command")
					}
				}
			}
			b.lock.Unlock()
		case <-ctx.Done():
			log.Info().Msg("closing bot")
			return
		}
	}
}

// send will send a message and store the appropriate trigger.
// it will automatically execute the default command if user does not reply.
// TODO : send confirmation of auto-invoke - use tgbotapi.Message here
func (b *Bot) send(idx api.Index, msg tgbotapi.MessageConfig, trigger *api.Trigger) (int, error) {
	// before sending check for blocked triggers ...
	if txt, ok := b.checkIfBlocked(trigger); ok {
		if bot, ok := b.chat[idx]; ok {
			sent, err := bot.b.Send(addLine(msg, txt))
			return sent.MessageID, err
		}
		return 0, fmt.Errorf("index not found: %v", idx)
	}
	// otherwise send the message and add the trigger
	// TODO : refactor and wrap up this logic
	t := defaultTimeout
	if trigger != nil {
		if trigger.Timeout > 0 {
			t = trigger.Timeout
		}
	}
	// TODO : be able to expose more details on the trigger
	if trigger != nil && len(trigger.Default) > 0 {
		msg = addLine(msg, fmt.Sprintf("[%s] %vs -> %v", trigger.Description, t.Seconds(), trigger.Default))
	}
	var sent tgbotapi.Message
	var err error
	if bot, ok := b.chat[idx]; ok {
		sent, err = bot.b.Send(msg)
		return sent.MessageID, err
	} else {
		err = fmt.Errorf("could not find index: %v", idx)
	}
	if err != nil {
		return 0, err
	}
	if trigger != nil && len(trigger.Default) > 0 {
		// we know we can send it back
		msgID := -1 * int(uuid.New().ID())

		return msgID, b.executeTrigger(*trigger,
			api.NewCommand(msgID, user.Bot, trigger.Default...))
	}
	return sent.MessageID, nil
}

// execute will try to find and execute the trigger attached to the replied message.
func (b *Bot) execute(private api.Index, message *tgbotapi.Message, replyID int) {
	// find the replyTo to the message
	if triggerID, ok := b.messages[replyID]; ok {
		// try to find trigger id if it s still valid
		if trigger, ok := b.triggers[triggerID]; ok {
			err := b.executeTrigger(*trigger, newCommand(message))
			if err != nil {
				b.Send(private, api.NewMessage("could not add trigger").AddLine(err.Error()).ReplyTo(message.MessageID), nil)
			}
		} else {
			// no trigger found for this id (could be already consumed)
			log.Debug().Int("id", replyID).Msg("trigger already applied")
			b.Send(private, api.NewMessage("trigger already applied").ReplyTo(message.MessageID), nil)
		}
		// clean up the initial message cache
		delete(b.messages, replyID)
	} else {
		log.Error().Int("id", replyID).Msg("trigger expired")
		b.Send(private, api.NewMessage("trigger expired").ReplyTo(message.MessageID), nil)
	}
}

// executeTrigger will execute the given trigger.
// it will make sure the block on the trigger and state regarding this event are handled accordingly.
func (b *Bot) executeTrigger(trigger api.Trigger, cmd api.Command) error {
	key := trigger.Key
	// we basically propagate the trigger to the right processor
	if ch, ok := b.consumers[key]; ok {
		// TODO : check for finding a consistent way on the dummy messageIDs
		ch <- cmd
		return nil
	}
	delete(b.triggers, trigger.ID)
	return fmt.Errorf("could not find consumer: %v", key)
}

// newCommand creates a new command based on the input message
func newCommand(message *tgbotapi.Message) api.Command {
	txt := strings.Split(message.Text, " ")
	return api.NewCommand(message.MessageID, message.From.UserName, txt...)
}
