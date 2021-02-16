package telegram

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/api"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/rs/zerolog/log"
)

// NewMessage creates a new telegram message config
func NewMessage(message *api.Message) tgbotapi.MessageConfig {
	cID := os.Getenv(telegramChatID)
	tgChatID, err := strconv.ParseInt(cID, 10, 64)
	if err != nil {
		panic("invalid chat id")
	}
	msg := tgbotapi.NewMessage(tgChatID, message.Text)
	if message.Reply > 0 {
		msg.ReplyToMessageID = message.Reply
	}
	return msg
}

func addLine(msg tgbotapi.MessageConfig, txt string) tgbotapi.MessageConfig {
	msg.Text = fmt.Sprintf("%s\n%s", msg.Text, txt)
	return msg
}

// listenToUpdates listens to updates for the telegram bot.
func (b *Bot) listenToUpdates(ctx context.Context, updates tgbotapi.UpdatesChannel) {
	for {
		select {
		case update := <-updates:
			if update.Message == nil { // ignore any non-Message Updates
				continue
			}

			reply := update.Message.ReplyToMessage
			if reply != nil {
				// if it s a reply , we are responsible to act on the trigger
				b.execute(update.Message, reply)
				continue
			}

			var chatID int64
			if update.Message.Chat != nil {
				chatID = update.Message.Chat.ID
			}
			log.Info().
				Str("from", update.Message.From.UserName).
				Str("text", update.Message.Text).
				Int64("chat", chatID).
				Msg("message received")

			for k, consumer := range b.consumers {
				// propagate the message
				if strings.HasPrefix(update.Message.Text, k.prefix) {
					select {
					case consumer <- api.Command{
						ID:      update.Message.MessageID,
						User:    update.Message.From.UserName,
						Content: update.Message.Text,
					}:
					case <-time.After(1 * time.Second):
						log.Warn().Str("prefix", k.prefix).Str("consumer", fmt.Sprintf("%+v", k)).Str("command", update.Message.Text).Msg("consumer did not receive command")
					}
				}
			}
		case <-ctx.Done():
			log.Info().Msg("closing bot")
			return
		}
	}
}

// deferExecute plans the execution of the given trigger (with the defaults)
// at the specified timeout
func (b *Bot) deferExecute(timeout time.Duration, replyID int, trigger *api.Trigger) {
	<-time.After(timeout)
	if trigger, ok := b.triggers[trigger.ID]; ok {
		if txt, ok := b.checkIfBlocked(trigger); ok {
			b.Send(api.NewMessage(txt).ReplyTo(replyID), nil)
			return
		}
		b.executeTrigger(trigger, api.Command{
			Content: trigger.Default[0],
		}, trigger.Default[1:]...)
	}
	delete(b.triggers, trigger.ID)
	delete(b.messages, replyID)
}

// execute will try to find and execute the trigger attached to the replied message.
func (b *Bot) execute(message, replyTo *tgbotapi.Message) {
	// find the replyTo to the message
	println(fmt.Sprintf("message = %+v", message))
	if triggerID, ok := b.messages[replyTo.MessageID]; ok {
		// try to find trigger id if it s still valid
		println(fmt.Sprintf("triggerID = %+v", triggerID))
		if trigger, ok := b.triggers[triggerID]; ok {
			cmd, opts := api.ParseCommand(message.MessageID, message.From.UserName, message.Text)
			b.executeTrigger(trigger, cmd, opts...)
		} else {
			// no trigger found for this id (could be already consumed)
			log.Debug().Int("id", replyTo.MessageID).Msg("trigger already applied")
			b.Send(api.NewMessage("trigger already applied").ReplyTo(message.MessageID), nil)
		}
		// clean up the initial message cache
		delete(b.messages, replyTo.MessageID)
	} else {
		log.Error().Int("id", replyTo.MessageID).Msg("trigger expired")
		b.Send(api.NewMessage("trigger expired").ReplyTo(message.MessageID), nil)
	}
}

// executeTrigger will execute the given trigger.
// it will make sure the block on the trigger and state regarding this event are handled accordingly.
func (b *Bot) executeTrigger(trigger *api.Trigger, cmd api.Command, opts ...string) {
	rsp, err := trigger.Exec(cmd, opts...)
	if err != nil {
		b.Send(api.NewMessage(fmt.Sprintf("[trigger] error: %v", err)), nil)
		return
	}
	// TODO : check how and when to delete the triggers
	delete(b.triggers, trigger.ID)
	// block for coming triggers of the same kind
	b.blockedTriggers[trigger.ID] = time.Now()
	// remove the trigger if its not used
	go func() {
		<-time.After(blockTimeout)
		delete(b.blockedTriggers, trigger.ID)
	}()
	b.Send(api.NewMessage(fmt.Sprintf("[trigger] completed: %v", cmd)).AddLine(rsp), nil)
}
