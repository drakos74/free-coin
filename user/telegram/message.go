package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/api"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/rs/zerolog/log"
)

// NewMessage creates a new telegram message config
func (b *Bot) newMessage(private api.Index, message *api.Message) tgbotapi.MessageConfig {
	tgChatID := b.publicChatID
	if private {
		tgChatID = b.privateChatID
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
func (b *Bot) listenToUpdates(ctx context.Context, private api.Index, updates tgbotapi.UpdatesChannel) {
	for {
		select {
		case update := <-updates:
			if update.Message == nil { // ignore any non-Message Updates
				continue
			}

			reply := update.Message.ReplyToMessage
			if reply != nil {
				log.Info().
					Str("from", update.Message.From.UserName).
					Str("text", update.Message.Text).
					Int64("chat", update.Message.Chat.ID).
					Str("private", fmt.Sprintf("%v", private)).
					Int("messageID", reply.MessageID).
					Msg("reply received")
				// if it s a reply , we are responsible to act on the trigger
				b.process <- executableTrigger{
					message: update.Message,
					replyID: reply.MessageID,
					private: private,
				}
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
				Str("private", fmt.Sprintf("%v", private)).
				Msg("message received")

			for k, consumer := range b.consumers {
				// propagate the message
				if strings.HasPrefix(update.Message.Text, k.prefix) {
					log.Info().
						Str("from", update.Message.From.UserName).
						Str("text", update.Message.Text).
						Str("consumer", fmt.Sprintf("%+v", k)).
						Msg("message propagated")
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

// processExecution listens to execution commands.
// this is to synchronise all commands in a single routine and avoid lock contention and race conditions.
func (b *Bot) processExecution() {
	for exec := range b.process {
		if exec.message != nil && exec.trigger != nil {
			// just to handle the adding of the trigger
			b.messages[exec.message.MessageID] = exec.trigger.ID
			b.triggers[exec.trigger.ID] = exec.trigger
			continue
		}
		if exec.message != nil {
			b.execute(exec.private, exec.message, exec.replyID)
		} else if exec.trigger != nil {
			b.deferExecute(exec.private, exec.trigger, exec.replyID)
		} else {
			panic("invalid executable trigger received")
		}
	}
}

// send will send a message and store the appropriate trigger.
// it will automatically execute the default command if user does not reply.
// TODO : send confirmation of auto-invoke - use tgbotapi.Message here
func (b *Bot) send(private api.Index, msg tgbotapi.MessageConfig, trigger *api.Trigger) (int, error) {
	// before sending check for blocked triggers ...
	if txt, ok := b.checkIfBlocked(trigger); ok {
		if private {
			sent, err := b.privateBot.Send(addLine(msg, txt))
			return sent.MessageID, err
		}
		sent, err := b.publicBot.Send(addLine(msg, txt))
		return sent.MessageID, err
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
	if private {
		sent, err = b.privateBot.Send(msg)
	} else {
		sent, err = b.publicBot.Send(msg)
	}
	if err != nil {
		return 0, err
	}
	if trigger != nil {
		if len(trigger.Default) > 0 {
			go func() {
				<-time.After(t)
				b.process <- executableTrigger{
					trigger: trigger,
					replyID: sent.MessageID,
					private: private,
				}
			}()
		}
	}
	return sent.MessageID, nil
}

// deferExecute plans the execution of the given trigger (with the defaults)
// at the specified timeout
func (b *Bot) deferExecute(private api.Index, trigger *api.Trigger, replyID int) {
	if trigger, ok := b.triggers[trigger.ID]; ok {
		if txt, ok := b.checkIfBlocked(trigger); ok {
			b.Send(private, api.NewMessage(txt).ReplyTo(replyID), nil)
			return
		}
		b.executeTrigger(private, trigger, api.Command{
			Content: strings.Join(trigger.Default, " "),
		})
	}
	delete(b.triggers, trigger.ID)
	delete(b.messages, replyID)
}

// execute will try to find and execute the trigger attached to the replied message.
func (b *Bot) execute(private api.Index, message *tgbotapi.Message, replyID int) {
	// find the replyTo to the message
	if triggerID, ok := b.messages[replyID]; ok {
		// try to find trigger id if it s still valid
		if trigger, ok := b.triggers[triggerID]; ok {
			cmd := api.ParseCommand(message.MessageID, message.From.UserName, message.Text)
			b.executeTrigger(private, trigger, cmd)
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
func (b *Bot) executeTrigger(private api.Index, trigger *api.Trigger, cmd api.Command) {
	rsp, err := trigger.Exec(cmd)
	if err != nil {
		b.Send(private, api.NewMessage(fmt.Sprintf("[trigger] error: %v", err)), nil)
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
	b.Send(private, api.NewMessage(fmt.Sprintf("[trigger] completed: %v", cmd)).AddLine(rsp), nil)
}
