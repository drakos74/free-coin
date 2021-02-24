package processor

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/drakos74/free-coin/internal/model"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestPosition_TradeProcessing(t *testing.T) {
	testTradeProcessing(t, Position)
}

func TestPosition_Update(t *testing.T) {

	type test struct {
		msg       []string
		cmd       string
		positions []model.Position
		reply     []string
		replyMsg  []string
		fp        int
	}

	tests := map[string]test{
		"no-positions": {
			cmd: "?p",
			msg: []string{noPositionMsg},
		},
		"one-positions": {
			cmd:       "?p",
			msg:       []string{string(model.BTC)},
			positions: []model.Position{model.NewPosition(*mockTrade(model.BTC, model.Buy), 1)},
			fp:        1,
		},
		"many-positions": {
			cmd: "?p",
			msg: []string{string(model.BTC), string(model.ETH), string(model.LINK)},
			positions: []model.Position{
				model.NewPosition(*mockTrade(model.BTC, model.Buy), 0.5),
				model.NewPosition(*mockTrade(model.ETH, model.Sell), 1),
				model.NewPosition(*mockTrade(model.LINK, model.Buy), 100),
			},
			fp: 3,
		},
		"close-positions": {
			cmd:       "?p",
			msg:       []string{string(model.BTC)},
			positions: []model.Position{model.NewPosition(*mockTrade(model.BTC, model.Buy), 1)},
			reply:     []string{"close"},
			replyMsg:  []string{"close position for BTC"},
		},
		"reverse-positions": {
			cmd:       "?p",
			msg:       []string{string(model.ETH)},
			positions: []model.Position{model.NewPosition(*mockTrade(model.ETH, model.Buy), 1)},
			reply:     []string{"reverse"},
			replyMsg:  []string{"reverse position for ETH"},
			// note we get 0 ,  because we have not implemented the reverse close for our test.
			// this is exchange specific.
			fp: 0,
		},
		"extend-positions": {
			cmd:       "?p",
			msg:       []string{string(model.LINK)},
			positions: []model.Position{model.NewPosition(*mockTrade(model.LINK, model.Buy), 1)},
			reply:     []string{"extend 2 2"},
			replyMsg:  []string{"extend position for LINK"},
			fp:        1,
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
					err := client.OpenPosition(p)
					assert.NoError(t, err)
				}
			}

			cmds, msgs := run(client, in, out, Position)

			wg := new(sync.WaitGroup)
			if len(tt.positions) == 0 {
				wg.Add(1)
			}

			go func() {
				i := 0
				for msg := range msgs {
					assert.Contains(t, msg.msg.Text, tt.msg[i])
					if tt.reply != nil && len(tt.reply) > i {
						userCommand := tt.reply[i]
						if userCommand != "" {
							// emulate the user replying ...
							cmd := api.NewCommand(0, "iam", userCommand)
							reply, err := msg.trigger.Exec(cmd)
							assert.NoError(t, err)
							// we know that our default mock positions are on btc
							assert.Contains(t, reply, tt.replyMsg[i])
							println(fmt.Sprintf("user.Reply = %+v", reply))
						}
					}
					wg.Done()
					i++
				}
			}()

			command := api.NewCommand(0, "iam", tt.cmd)
			cmds <- command

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
					err := client.OpenPosition(p)
					assert.NoError(t, err)
				}
			}

			_, msgs := run(client, in, out, Position)
			go func() {
				for msg := range msgs {
					println(fmt.Sprintf("msg.msg.Text = %+v", msg.msg.Text))
					time.Sleep(100 * time.Millisecond)
					if len(msg.trigger.Default) > 0 {
						// TODO : assert on the closing profit
						reply, err := msg.trigger.Exec(api.NewCommand(1, "", strings.Join(msg.trigger.Default, " ")))
						if err == nil {
							println(fmt.Sprintf("reply = %+v", reply))
						}
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

			wg.Wait()

		})
	}
}
