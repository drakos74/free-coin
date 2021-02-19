package processor

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/algo/model"

	"github.com/drakos74/free-coin/internal/api"
)

func testTradeProcessing(t *testing.T, processor func(client model.TradeClient, user model.UserInterface) api.Processor) {

	_, in, out, _, _ := run(processor)

	num := 1000
	wg := new(sync.WaitGroup)
	wg.Add(num)
	go func() {
		start := time.Now()
		for i := 0; i < num; i++ {
			trade := newTrade(api.BTC, 30000, 1, api.Buy, start.Add(1*time.Second))
			in <- trade
		}
	}()

	go func() {
		for range out {
			wg.Done()
		}
	}()

	wg.Wait()
}

func run(processor func(client model.TradeClient, user model.UserInterface) api.Processor) (client model.TradeClient, in, out chan *api.Trade, commands chan api.Command, confirms chan sendAction) {
	client = newMockClient()

	commands = make(chan api.Command)
	confirms = make(chan sendAction)
	user := newMockUser(commands, confirms)

	in = make(chan *api.Trade)
	out = make(chan *api.Trade)

	go processor(client, user)(in, out)

	return
}

type sendAction struct {
	msg     *api.Message
	trigger *api.Trigger
}

type mockUser struct {
	sent   chan sendAction
	action chan api.Command
}

func newMockUser(commands chan api.Command, confirm chan sendAction) *mockUser {
	return &mockUser{
		sent:   confirm,
		action: commands,
	}
}

func (u *mockUser) Run(ctx context.Context) error {
	panic("implement me")
}

func (u *mockUser) Listen(key, prefix string) <-chan api.Command {
	return u.action
}

func (u *mockUser) Send(message *api.Message, trigger *api.Trigger) int {
	u.sent <- sendAction{
		msg:     message,
		trigger: trigger,
	}
	return 0
}

type mockClient struct {
	positions []api.Position
}

func newMockClient() *mockClient {
	return &mockClient{positions: make([]api.Position, 0)}
}

func (c *mockClient) Trades(stop <-chan struct{}, coin api.Coin, stopExecution api.Condition) (api.TradeSource, error) {
	panic("implement me")
}

func (c *mockClient) OpenPositions(ctx context.Context) (*api.PositionBatch, error) {
	return &api.PositionBatch{
		Positions: c.positions,
		Index:     0,
	}, nil
}

func (c *mockClient) OpenPosition(position api.Position) error {
	c.positions = append(c.positions, position)
	return nil
}

func (c *mockClient) ClosePosition(position api.Position) error {
	if position.ID == "" {
		// for now, just fail if we dont handle normal ids
		// without them we might miss some inconsistency
		return fmt.Errorf("test without ID is invalid")
	}
	pp := make([]api.Position, 0)
	var removed bool
	for _, p := range c.positions {
		if p.ID != position.ID {
			pp = append(pp, p)
		} else {
			removed = true
		}
	}
	c.positions = pp
	if removed {
		return nil
	}
	return fmt.Errorf("could not remove position")
}

func mockTrade(c api.Coin, t api.Type) *api.Trade {
	return &api.Trade{
		Coin:   c,
		Price:  400000,
		Volume: 1,
		Time:   time.Now(),
		Type:   t,
		Meta:   make(map[string]interface{}),
	}
}

func newTrade(c api.Coin, price, volume float64, t api.Type, time time.Time) *api.Trade {
	return &api.Trade{
		Coin:   c,
		Price:  price,
		Volume: volume,
		Time:   time,
		Type:   t,
		Meta:   make(map[string]interface{}),
	}
}
