package trader

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	mockClient "github.com/drakos74/free-coin/client/local"
	"github.com/drakos74/free-coin/internal/concurrent"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	mockUser "github.com/drakos74/free-coin/user/local"
	"github.com/stretchr/testify/assert"
)

// TODO : registry error

func TestSignalProcessorBasic(t *testing.T) {

	type test struct {
		storeConstr    func(store storage.Persistence) storage.Shard
		registryConstr func(store storage.Registry) storage.EventRegistry
		startErr       bool
		messages       []*model.TrackedOrder
		userMessages   int
		orders         int
	}

	now := time.Now()

	tests := map[string]test{
		"reg-err": {
			registryConstr: errRegistry,
			startErr:       true,
			messages: []*model.TrackedOrder{
				newOrder(now, model.Buy, "MANUAL"),
				newOrder(now, model.Sell, "MANUAL"),
			},
		},
		"st-err": {
			storeConstr: errShard,
			startErr:    true,
			messages: []*model.TrackedOrder{
				newOrder(now, model.Buy, "MANUAL"),
				newOrder(now, model.Sell, "MANUAL"),
			},
		},
		"noop": {
			messages: []*model.TrackedOrder{
				newOrder(now, model.Sell, "MANUAL"),
			},
			userMessages: 2,
		},
		"buy": {
			messages: []*model.TrackedOrder{
				newOrder(now, model.Buy, "MANUAL"),
			},
			userMessages: 3,
			orders:       1,
		},
		"ignore-re-buy": {
			messages: []*model.TrackedOrder{
				newOrder(now, model.Buy, "MANUAL"),
				newOrder(now.Add(time.Minute), model.Buy, "MANUAL"),
			},
			userMessages: 3,
			orders:       1,
		},
		"close": {
			messages: []*model.TrackedOrder{
				newOrder(now, model.Buy, "MANUAL"),
				newOrder(now.Add(time.Hour), model.Sell, "BS"),
			},
			userMessages: 4,
			orders:       2,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {

			// init with the defaults
			store := storage.NewMockStorage()
			if tt.storeConstr == nil {
				tt.storeConstr = normalShard
			}
			if tt.registryConstr == nil {
				tt.registryConstr = normalRegistry
			}
			registry := storage.NewMockRegistry()

			// init the mock client and user interface
			client := mockClient.NewExchange("")
			user := mockUser.NewMockUser()

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

			processor := Trade(
				"demo",
				tt.storeConstr(store),
				tt.registryConstr(registry),
				client,
				user,
				Config{Settings: make(map[model.Coin]map[time.Duration]Settings)},
			)

			if !tt.startErr {
				// wait for the processor to be turned on
				wg.Wait()
			}

			// start the processor
			in := make(chan *model.TrackedOrder)
			out := make(chan *model.TrackedOrder)
			go processor(in, out)

			// make sure we processed all messages
			signalMessages := concurrent.NewAssertion(len(tt.messages))
			// start the consumer
			go func() {
				for message := range out {
					signalMessages.Expect(message)
				}
			}()

			// start the signal processor
			if !tt.startErr {
				wg.Add(1)
				go user.MustMockMessage("?r start", "demo", "")
				// wait for the processor to have received the start command,
				// before pushing signals
				wg.Wait()
			}
			// just disable the waitGroup for the next messages.
			// TODO : maybe find a better way (?)
			wg.Add(1000)

			// feed the input
			for _, message := range tt.messages {
				in <- message
			}

			signalMessages.Assert(t)
			userMessages.Assert(t)

			orders := client.Orders()
			assert.Equal(t, tt.orders, len(orders))

			for k, event := range registry.Events {
				fmt.Printf("event = %s - %+v\n", k, event)
			}

		})
	}
}

func newOrder(now time.Time, t model.Type, strategy string) *model.TrackedOrder {
	return model.NewOrder("").
		WithVolume(1).
		Market().
		WithType(t).
		CreateTracked(model.NewKey("", time.Minute, strategy), now)
}

func normalShard(store storage.Persistence) storage.Shard {
	return func(shard string) (storage.Persistence, error) {
		return store, nil
	}
}

func errShard(_ storage.Persistence) storage.Shard {
	return func(shard string) (storage.Persistence, error) {
		return nil, errors.New("error")
	}
}

func normalRegistry(registry storage.Registry) storage.EventRegistry {
	return func(path string) (storage.Registry, error) {
		return registry, nil
	}
}

func errRegistry(_ storage.Registry) storage.EventRegistry {
	return func(path string) (storage.Registry, error) {
		return nil, errors.New("error")
	}
}
