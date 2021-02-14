package telegram

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/drakos74/free-coin/coinapi"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type mockBot struct {
	input  chan tgbotapi.Update
	output chan tgbotapi.MessageConfig
}

func (m *mockBot) GetUpdatesChan(config tgbotapi.UpdateConfig) (tgbotapi.UpdatesChannel, error) {
	return m.input, nil
}

func (m *mockBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	if msg, ok := c.(tgbotapi.MessageConfig); ok {
		m.output <- msg
	}
	return tgbotapi.Message{}, nil
}

func newMockBot(input chan tgbotapi.Update, output chan tgbotapi.MessageConfig) *Bot {
	return &Bot{
		bot:             &mockBot{input: input, output: output},
		messages:        make(map[int]string),
		triggers:        make(map[string]*coinapi.Trigger),
		blockedTriggers: make(map[string]time.Time),
		consumers:       make(map[consumerKey]chan coinapi.Command),
	}
}

func TestBot_Listen(t *testing.T) {

	in := make(chan tgbotapi.Update)
	out := make(chan tgbotapi.MessageConfig)
	b := newMockBot(in, out)
	ctx, cnl := context.WithCancel(context.Background())
	err := b.Run(ctx)
	assert.NoError(t, err)

	cc := b.Listen("my-consumer", "1")

	var wg sync.WaitGroup
	wg.Add(10)

	producerCount := 0
	validMessageCount := 0
	go func() {
		for {
			index := rand.Intn(3)
			in <- tgbotapi.Update{
				Message: &tgbotapi.Message{
					MessageID: rand.Int(),
					From: &tgbotapi.User{
						UserName: "@user",
					},
					Text: fmt.Sprintf("%d ... my-message", index),
				},
			}
			producerCount++
			if index == 1 {
				validMessageCount++
				if validMessageCount >= 10 {
					close(in)
					return
				}
			}
		}
	}()

	count := 0
	go func() {
		for c := range cc {
			// check we only got messages with our predefined prefix
			assert.True(t, strings.HasPrefix(c.Content, "1"))
			count++
			wg.Done()
		}
	}()

	wg.Wait()
	cnl()

	assert.True(t, producerCount > count)
	assert.Equal(t, validMessageCount, count)

}

func TestBot_SendTrigger(t *testing.T) {

	defaultTimeout = 1 * time.Second

	type test struct {
		trigger  *coinapi.Trigger
		msgCount int
		count    int
		tick     <-chan time.Time
		reply    func(msg tgbotapi.MessageConfig, in chan tgbotapi.Update)
		msgs     []string
	}

	tests := map[string]test{
		"auto-trigger": {
			trigger: coinapi.NewTrigger(func(command coinapi.Command, options ...string) (string, error) {
				return command.Content, nil
			}),
			msgCount: 1,
			count:    1,
			tick:     time.Tick(5 * time.Second),
			msgs:     []string{"text"},
		},
		"no-auto-trigger": {
			trigger: coinapi.NewTrigger(func(command coinapi.Command, options ...string) (string, error) {
				// options[0] should be the 2nd argument of the defaults
				return command.Content, nil
			}).WithDefaults("cc").WithTimeout(1 * time.Second),
			msgCount: 1,
			// we get 2 messages here, because we get the trigger complete confirmation
			count: 2,
			tick:  time.Tick(5 * time.Second),
			// NOTE : we get 'cc' because the default was triggered
			msgs: []string{"text", "cc"},
		},
		"manual-trigger": {
			trigger: coinapi.NewTrigger(func(command coinapi.Command, options ...string) (string, error) {
				return command.Content, nil
			}).WithDefaults("cc").WithTimeout(30 * time.Second),
			msgCount: 1,
			// we get 2 because we also get the trigger expired message when we try to reply
			count: 2,
			tick:  time.Tick(5 * time.Second),
			reply: func(msg tgbotapi.MessageConfig, in chan tgbotapi.Update) {
				in <- tgbotapi.Update{
					Message: &tgbotapi.Message{
						MessageID: 0,
						From: &tgbotapi.User{
							UserName: "@user",
						},
						Text: "dd",
						ReplyToMessage: &tgbotapi.Message{
							MessageID: msg.ReplyToMessageID,
						},
					},
				}
			},
			// NOTE : we get 'dd' , because this was triggered from the reply and not the default.
			msgs: []string{"text", "dd"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			in := make(chan tgbotapi.Update)
			out := make(chan tgbotapi.MessageConfig)
			b := newMockBot(in, out)
			ctx, cnl := context.WithCancel(context.Background())
			err := b.Run(ctx)
			assert.NoError(t, err)

			var wg sync.WaitGroup
			wg.Add(tt.count)

			msgs := make([]string, 0)

			count := 0
			go func() {
				for {
					select {
					case msg := <-out:
						msgs = append(msgs, msg.Text)
						count++
						wg.Done()
						// reply , but not during message confirmation
						if tt.reply != nil && !strings.HasPrefix(msg.Text, "[trigger]") {
							tt.reply(msg, in)
						}
					case <-tt.tick:
						t.Fail()
						wg.Done()
						return
					}
				}
			}()

			for i := 0; i < tt.msgCount; i++ {
				// send a message to the user
				b.Send(coinapi.NewMessage("text"), tt.trigger)
			}

			wg.Wait()
			cnl()

			for i, m := range msgs {
				assert.True(t, strings.Contains(m, tt.msgs[i]))
			}
			assert.Equal(t, tt.count, count)
		})
	}

}
