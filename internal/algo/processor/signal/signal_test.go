package signal

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	mockClient "github.com/drakos74/free-coin/client/local"
	"github.com/drakos74/free-coin/internal/concurrent"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	mockUser "github.com/drakos74/free-coin/user/local"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

func TestSignalProcessor(t *testing.T) {

	type test struct {
		trades       []model.Trade
		messages     []Message
		userMessages int
		orders       int
	}

	now := time.Now()

	tests := map[string]test{
		"noop": {
			trades: make([]model.Trade, 0),
			messages: []Message{
				NewMessage(now, model.Sell, "MANUAL"),
			},
			userMessages: 2,
		},
		"buy": {
			trades: make([]model.Trade, 0),
			messages: []Message{
				NewMessage(now, model.Buy, "MANUAL"),
			},
			userMessages: 3,
			orders:       1,
		},
		"ignore-re-buy": {
			trades: make([]model.Trade, 0),
			messages: []Message{
				NewMessage(now, model.Buy, "MANUAL"),
				NewMessage(now.Add(time.Minute), model.Buy, "MANUAL"),
			},
			userMessages: 3,
			orders:       1,
		},
		"close": {
			trades: make([]model.Trade, 0),
			messages: []Message{
				NewMessage(now, model.Buy, "MANUAL"),
				NewMessage(now.Add(time.Hour), model.Sell, "BS"),
			},
			userMessages: 4,
			orders:       2,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {

			store := storage.NewMockStorage()
			var shard storage.Shard = func(shard string) (storage.Persistence, error) {
				return store, nil
			}

			mockRegistry := storage.NewMockRegistry()
			var registry storage.EventRegistry = func(path string) (storage.Registry, error) {
				return mockRegistry, nil
			}
			client := mockClient.NewExchange("")
			user := mockUser.NewMockUser()
			signals := NewSignalChannel(make(chan Message))

			// we expect 2 messages
			// 1 for the initialisation
			// 1 for the start command
			userMessages := concurrent.NewAssertion(tt.userMessages)
			// enable the user messaging pipeline
			wg := new(sync.WaitGroup)
			wg.Add(1)

			// make sure we wait for the go routine to initialise.
			// before we proceed
			concurrent.Async(func() {
				for message := range user.Messages {
					wg.Done()
					userMessages.Expect(message)
				}
			})

			processor := Receiver("demo", shard, registry, client, user, signals, nil)
			// wait for the processor to be turned on
			wg.Wait()

			// start the processor
			in := make(chan *model.Trade)
			out := make(chan *model.Trade)
			go processor(in, out)

			// make sure we processed all messages
			signalMessages := concurrent.NewAssertion(len(tt.messages))
			// start the consumer
			go func() {
				for message := range signals.Output {
					signalMessages.Expect(message)
				}
			}()

			// start the signal processor
			wg.Add(1)
			go user.MockMessage("?r start", "demo", "")
			// wait for the processor to have received the start command,
			// before pushing signals
			wg.Wait()
			// just disable the waitGroup for the next messages.
			// TODO : maybe find a better way (?)
			wg.Add(1000)

			// feed the input
			for _, message := range tt.messages {
				signals.Source <- message
			}

			signalMessages.Assert(t)
			userMessages.Assert(t)

			orders := client.Orders()
			assert.Equal(t, tt.orders, len(orders))

			for k, event := range mockRegistry.Events {
				fmt.Printf("event = %s - %+v\n", k, event)
			}

		})
	}

}

func NewMessage(now time.Time, t model.Type, strategy string) Message {
	action := Action{
		Buy:        "0",
		StrongBuy:  "0",
		Sell:       "1",
		StrongSell: "0",
	}
	if t == model.Buy {
		action = Action{
			Buy:        "1",
			StrongBuy:  "0",
			Sell:       "0",
			StrongSell: "0",
		}
	}

	return Message{
		Config: Config{
			SA:       "15",
			Interval: "60m",
			Mode:     strategy,
		},
		Signal: action,
		Data: Data{
			Exchange: "BINANCE",
			Ticker:   "BTC",
			Price:    "50000",
			Volume:   "20",
			TimeNow:  now.Format(time.RFC3339),
		},
	}
}
