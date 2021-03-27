package test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/algo/processor/position"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestPosition_TradeProcessing(t *testing.T) {
	testTradeProcessing(t, func(client api.Exchange, user api.User) api.Processor {
		return position.Position(storage.VoidShard(""), storage.NewVoidRegistry(), client, user, api.NewBlock(), testConfig(model.BTC, processor.Config{}))
	})
}

func TestPosition_Update(t *testing.T) {
	type test struct {
		msg         []string
		cmd         string
		positions   []model.Position
		reply       []string
		fp          int
		expMessages int
	}

	tests := map[string]test{
		"no-positions": {
			cmd: "?p",
			msg: []string{position.NoPositionMsg},
		},
		"one-positions": {
			cmd:       "?p",
			msg:       []string{string(model.BTC)},
			positions: []model.Position{mockPosition("ID", model.BTC, model.Buy)},
			fp:        1,
		},
		"many-positions": {
			cmd: "?p",
			msg: []string{string(model.BTC), string(model.ETH), string(model.LINK)},
			positions: []model.Position{
				mockPosition("1", model.BTC, model.Buy),
				mockPosition("2", model.ETH, model.Sell),
				mockPosition("3", model.LINK, model.Buy),
			},
			fp: 3,
		},
		"close-positions": {
			cmd:         "?p",
			msg:         []string{string(model.BTC), "closed BTC"},
			positions:   []model.Position{mockPosition("ID", model.BTC, model.Buy)},
			reply:       []string{"?p BTC close ID"},
			expMessages: 2,
		},
		//"reverse-positions": { // TODO : fix this
		//	cmd:       "?p",
		//	msg:       []string{string(model.ETH), "unknown command: reverse"},
		//	positions: []model.Position{mockPosition("{ID}", model.ETH, model.Buy)},
		//	reply:     []string{"?p reverse"},
		//	// note we get 0 ,  because we have not implemented the reverse close for our test.
		//	// this is exchange specific.
		//	fp: 0,
		//},
		//"extend-positions": {
		//	cmd:       "?p",
		//	msg:       []string{string(model.LINK), "extend position for LINK"},
		//	positions: []model.Position{mockPosition("ID", model.LINK, model.Buy)},
		//	reply:     []string{"?p extend 2 2"},
		//	fp:        1,
		//},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			in := make(chan *model.Trade)
			out := make(chan *model.Trade)
			client := newMockClient()

			// add some positions
			if tt.positions != nil {
				for _, p := range tt.positions {
					_, err := client.OpenOrder(model.TrackedOrder{Order: model.FromPosition(p, false)})
					assert.NoError(t, err)
				}
			}

			incoming, outgoing := run(client, in, out, newPositionProcessor)

			wg := new(sync.WaitGroup)
			wg.Add(tt.expMessages)
			if len(tt.positions) == 0 {
				wg.Add(1)
			}

			go func() {
				i := 0
				for msg := range outgoing {
					println(fmt.Sprintf("msg.Text = %+v", msg.msg.Text))
					assert.Contains(t, msg.msg.Text, tt.msg[i])
					if tt.reply != nil && len(tt.reply) > i {
						userCommand := tt.reply[i]
						if userCommand != "" {
							println(fmt.Sprintf("userCommand = %+v", userCommand))
							// emulate the user replying ...
							cmd := api.NewCommand(0, "iam", userCommand)
							incoming <- cmd
						}
					}
					wg.Done()
					i++
				}
			}()

			command := api.NewCommand(0, "iam", tt.cmd)
			incoming <- command

			wg.Wait()

			// check the positions once more
			finalPositions, err := client.OpenPositions(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, tt.fp, len(finalPositions.Positions))
			for _, p := range finalPositions.Positions {
				println(fmt.Sprintf("p = %+v", p))
			}
		})
	}

}

// TODO : fix this test
func TestPosition_Track(t *testing.T) {
	type test struct {
		positions []model.Position
		transform func(i int) float64
		profit    float64
	}

	tests := map[string]test{
		"normal-loss": { // we should get messages continously
			positions: []model.Position{model.NewPosition(*mockTrade(model.BTC, model.Buy), 1)},
			transform: func(i int) float64 {
				return basePrice - 601 // we are right at the -1,5% stop loss
			},
			profit: -1.5,
		},
		"increasing-loss": { // we should get messages continously
			positions: []model.Position{model.NewPosition(*mockTrade(model.BTC, model.Buy), 1)},
			transform: func(i int) float64 {
				return basePrice - 100*float64(i)
			},
			profit: -4.5,
		},
		"closing-improving-loss": { // we should only get one message
			positions: []model.Position{model.NewPosition(*mockTrade(model.BTC, model.Buy), 1)},
			transform: func(i int) float64 {
				return (basePrice - 10000) + 100*float64(i)
			},
		},
		"closing-better-loss": { // we should get only a few messages
			positions: []model.Position{model.NewPosition(*mockTrade(model.BTC, model.Buy), 1)},
			transform: func(i int) float64 {
				if i > 50 {
					return basePrice - 100*float64(i)
				}
				return (basePrice - 10000) + 100*float64(i)
			},
			profit: -15.25,
		},
		"increasing-profit": { // we should get no message as we are doing continously profit
			positions: []model.Position{model.NewPosition(*mockTrade(model.BTC, model.Buy), 1)},
			transform: func(i int) float64 {
				return basePrice + 100*float64(i)
			},
		},
		"trailing-loss-profit": { // we should close position at a good level
			positions: []model.Position{model.NewPosition(*mockTrade(model.BTC, model.Buy), 1)},
			transform: func(i int) float64 {
				if i > 50 {
					return basePrice + 100*50.0 - 100*float64(i-50)
				}
				return basePrice + 100*float64(i)
			},
			profit: 9.25,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			in := make(chan *model.Trade)
			out := make(chan *model.Trade)
			client := newMockClient()

			// add some positions
			if tt.positions != nil {
				for _, p := range tt.positions {
					_, err := client.OpenOrder(model.TrackedOrder{Order: model.FromPosition(p, false)})
					assert.NoError(t, err)
				}
			}

			_, msgs := run(client, in, out, newPositionProcessor)
			go func() {
				for msg := range msgs {
					println(fmt.Sprintf("msg.msg.Text = %+v", msg.msg.Text))
					time.Sleep(100 * time.Millisecond)
					if len(msg.trigger.Default) > 0 {
						println(fmt.Sprintf("msg.trigger.Default = %+v", msg.trigger.Default))
						// TODO : assert on the closing profit
						// TODO : fix this logic for the processor
						//reply, err := msg.trigger.Exec(api.NewCommand(1, "", strings.Join(msg.trigger.Default, " ")))
						//if err == nil {
						//	println(fmt.Sprintf("reply = %+v", reply))
						//}
					}

				}
			}()

			wg := new(sync.WaitGroup)
			wg.Add(100)
			// consume from the output
			go func() {
				for range out {
					// nothing to do
					wg.Done()
				}
			}()

			//we ll wait to consume all trades here
			start := time.Now()
			for i := 0; i < 100; i++ {
				// send a trade ...
				trade := newTrade(model.BTC, tt.transform(i), 1, model.Buy, start.Add(time.Duration(i*15)*time.Second))
				// enable trade to publish messages
				trade.Live = true
				trade.Active = true
				in <- trade
			}

			// TODO : assert the profit
			println(fmt.Sprintf("tt.profit = %+v", tt.profit))

			wg.Wait()

		})
	}
}

func mockPosition(id string, coin model.Coin, t model.Type) model.Position {
	pos := model.NewPosition(*mockTrade(coin, t), 1)
	pos.ID = id
	return pos
}

func newPositionProcessor(client api.Exchange, user api.User) api.Processor {
	config := processor.Config{
		Duration: 10,
		Strategies: []processor.Strategy{
			{
				Open: processor.Open{
					Value: 0,
					Limit: 0,
				},
				Close: processor.Close{
					Instant: true,
					Profit: processor.Setup{
						Min:   1.5,
						Trail: 0.15,
					},
					Loss: processor.Setup{
						Min: 1,
					},
				},
			},
		},
	}
	return position.Position(storage.VoidShard(""), storage.NewVoidRegistry(), client, user, api.NewBlock(), testConfig("", config))
}
