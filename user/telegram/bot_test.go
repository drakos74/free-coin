package telegram

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/api"

	"github.com/stretchr/testify/assert"

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
		publicBot:       &mockBot{input: input, output: output},
		privateBot:      &Void{},
		messages:        make(map[int]string),
		triggers:        make(map[string]*api.Trigger),
		blockedTriggers: make(map[string]time.Time),
		consumers:       make(map[api.ConsumerKey]chan api.Command),
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

	var producerCount int64
	var validMessageCount int64
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
			atomic.AddInt64(&producerCount, 1)
			if index == 1 {
				atomic.AddInt64(&validMessageCount, 1)
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

	assert.True(t, int(producerCount) > count)
	assert.Equal(t, int(validMessageCount), count)

}

func TestBot_SendTrigger(t *testing.T) {

	os.Setenv(telegramChatID, "-1")

	defaultTimeout = 1 * time.Second

	type test struct {
		trigger  *api.Trigger
		msgCount int
		count    int
		tick     <-chan time.Time
		reply    func(msg tgbotapi.MessageConfig, in chan tgbotapi.Update)
		msgs     []string
	}

	// TODO : fix the tests
	tests := map[string]test{
		"no-auto-trigger": {
			trigger: api.NewTrigger(api.ConsumerKey{
				ID:     "",
				Key:    "",
				Prefix: "",
			}),
			msgCount: 1,
			count:    1,
			tick:     time.Tick(5 * time.Second),
			msgs:     []string{"text"},
		},
		"auto-trigger": {
			trigger: api.NewTrigger(api.ConsumerKey{
				ID:     "",
				Key:    "",
				Prefix: "",
			}).WithDefaults("cc").WithTimeout(1 * time.Second),
			msgCount: 1,
			// we get 2 messages here, because we get the trigger complete confirmation
			count: 2,
			tick:  time.Tick(5 * time.Second),
			// NOTE : we get 'cc' because the default was triggered
			msgs: []string{"text", "cc"},
		},
		"manual-trigger": {
			trigger: api.NewTrigger(api.ConsumerKey{
				ID:     "",
				Key:    "",
				Prefix: "",
			}).WithDefaults("cc").WithTimeout(3 * time.Second),
			msgCount: 1,
			// we get 3 because we also get the trigger expired message when we try to reply
			count: 2,
			tick:  time.Tick(10 * time.Second),
			reply: func(msg tgbotapi.MessageConfig, in chan tgbotapi.Update) {
				in <- tgbotapi.Update{
					Message: &tgbotapi.Message{
						MessageID: rand.Int(),
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

			var count int64
			go func(t *testing.T, tt test) {
				for {
					select {
					case msg := <-out:
						msgs = append(msgs, msg.Text)
						atomic.AddInt64(&count, 1)
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
			}(t, tt)

			for i := 0; i < tt.msgCount; i++ {
				// send a message to the user
				b.Send(false, api.NewMessage("text"), tt.trigger)
			}

			wg.Wait()
			cnl()

			for i, m := range msgs {
				println(fmt.Sprintf("message %d = %+v", i, m))
				assert.True(t, strings.Contains(m, tt.msgs[i]))
			}
			assert.Equal(t, tt.count, int(count))
		})
	}

}
