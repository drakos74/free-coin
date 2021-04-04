package test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	cointime "github.com/drakos74/free-coin/internal/time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
)

func testTradeProcessing(t *testing.T, processor func(client api.Exchange, user api.User) api.Processor) {

	in := make(chan *model.Trade)
	out := make(chan *model.Trade)
	client := newMockClient()

	run(client, in, out, processor)

	num := 1000
	wg := new(sync.WaitGroup)
	wg.Add(num)
	go func() {
		start := time.Now()
		for i := 0; i < num; i++ {
			trade := newTrade(model.BTC, 30000, 1, model.Buy, start.Add(1*time.Second))
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

func run(client api.Exchange, in, out chan *model.Trade, processor func(client api.Exchange, user api.User) api.Processor) (incoming chan api.Command, outgoing chan sendAction) {

	incoming = make(chan api.Command)
	outgoing = make(chan sendAction, 10)
	user := newMockUser(incoming, outgoing)

	go processor(client, user)(in, out)

	return
}

func consumeMessages(name string, wg *sync.WaitGroup, msgs chan sendAction) {
	i := 0
	for msg := range msgs {
		println(fmt.Sprintf("%s: msg = %+v", name, msg.msg.Text))
		if wg != nil {
			wg.Done()
		}
		i++
		println(fmt.Sprintf("%s i = %+v", name, i))
	}
}

type sendAction struct {
	msg     *api.Message
	trigger *api.Trigger
}

type mockUser struct {
	outgoing chan sendAction
	incoming chan api.Command
}

func newMockUser(incoming chan api.Command, outgoing chan sendAction) *mockUser {
	return &mockUser{
		outgoing: outgoing,
		incoming: incoming,
	}
}

func (u *mockUser) Run(ctx context.Context) error {
	panic("implement me")
}

func (u *mockUser) Listen(key, prefix string) <-chan api.Command {
	return u.incoming
}

func (u *mockUser) Send(private api.Index, message *api.Message, trigger *api.Trigger) int {
	u.outgoing <- sendAction{
		msg:     message,
		trigger: trigger,
	}
	return 0
}

type mockClient struct {
	positions []model.Position
}

func newMockClient() *mockClient {
	return &mockClient{
		positions: make([]model.Position, 0),
	}
}

func (c *mockClient) Trades(process <-chan api.Action, query api.Query) (model.TradeSource, error) {
	panic("implement me")
}

func (c *mockClient) CurrentPrice(ctx context.Context) (map[model.Coin]model.CurrentPrice, error) {
	return make(map[model.Coin]model.CurrentPrice), nil
}

func (c *mockClient) OpenPositions(ctx context.Context) (*model.PositionBatch, error) {
	return &model.PositionBatch{
		Positions: c.positions,
		Index:     0,
	}, nil
}

func (c *mockClient) OpenOrder(order model.TrackedOrder) (model.TrackedOrder, []string, error) {
	c.positions = append(c.positions, model.OpenPosition(order))
	return model.TrackedOrder{}, []string{}, nil
}

func (c *mockClient) ClosePosition(position model.Position) error {
	if position.ID == "" {
		// for now, just fail if we dont handle normal ids
		// without them we might miss some inconsistency
		return fmt.Errorf("test without ID is invalid")
	}
	pp := make([]model.Position, 0)
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

func (c *mockClient) Balance(ctx context.Context, priceMap map[model.Coin]model.CurrentPrice) (map[model.Coin]model.Balance, error) {
	// TODO :
	return make(map[model.Coin]model.Balance), nil
}

const basePrice = 40000

func mockTrade(c model.Coin, t model.Type) *model.Trade {
	trade := model.Trade{
		Coin:   c,
		Price:  basePrice,
		Volume: 1,
		Time:   time.Now(),
		Type:   t,
	}
	return &trade
}

func newTrade(c model.Coin, price, volume float64, t model.Type, time time.Time) *model.Trade {
	trade := model.Trade{
		Coin:   c,
		Price:  price,
		Volume: volume,
		Time:   time,
		Type:   t,
	}
	return &trade
}

func testConfig(c model.Coin, config ...processor.Config) map[model.Coin]map[time.Duration]processor.Config {
	configs := make(map[model.Coin]map[time.Duration]processor.Config)
	for _, cfg := range config {
		dd := cointime.ToMinutes(cfg.Duration)
		configs[c][dd] = cfg
	}
	return configs
}
