package processor

import (
	"context"
	"fmt"
	"sync"
	"testing"

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
		positions []api.Position
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
			msg:       []string{string(api.BTC)},
			positions: []api.Position{api.NewPosition(*mockTrade(api.BTC, api.Buy), 1)},
			fp:        1,
		},
		"many-positions": {
			cmd: "?p",
			msg: []string{string(api.BTC), string(api.ETH), string(api.LINK)},
			positions: []api.Position{
				api.NewPosition(*mockTrade(api.BTC, api.Buy), 0.5),
				api.NewPosition(*mockTrade(api.ETH, api.Sell), 1),
				api.NewPosition(*mockTrade(api.LINK, api.Buy), 100),
			},
			fp: 3,
		},
		"close-positions": {
			cmd:       "?p",
			msg:       []string{string(api.BTC)},
			positions: []api.Position{api.NewPosition(*mockTrade(api.BTC, api.Buy), 1)},
			reply:     []string{"close"},
			replyMsg:  []string{"close position for BTC"},
		},
		"reverse-positions": {
			cmd:       "?p",
			msg:       []string{string(api.ETH)},
			positions: []api.Position{api.NewPosition(*mockTrade(api.ETH, api.Buy), 1)},
			reply:     []string{"reverse"},
			replyMsg:  []string{"reverse position for ETH"},
			// note we get 0 ,  because we have not implemented the reverse close for our test.
			// this is exchange specific.
			fp: 0,
		},
		"extend-positions": {
			cmd:       "?p",
			msg:       []string{string(api.LINK)},
			positions: []api.Position{api.NewPosition(*mockTrade(api.LINK, api.Buy), 1)},
			reply:     []string{"extend 2 2"},
			replyMsg:  []string{"extend position for LINK"},
			fp:        1,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			client, _, _, cmds, msgs := run(Position)

			wg := new(sync.WaitGroup)
			if len(tt.positions) == 0 {
				wg.Add(1)
			}
			// add some positions
			if tt.positions != nil {
				for _, p := range tt.positions {
					wg.Add(1)
					err := client.OpenPosition(p)
					assert.NoError(t, err)
				}
			}

			go func() {
				i := 0
				for msg := range msgs {
					assert.Contains(t, msg.msg.Text, tt.msg[i])
					println(fmt.Sprintf("user.Text = %+v", msg.msg.Text))
					if tt.reply != nil && len(tt.reply) > i {
						userCommand := tt.reply[i]
						if userCommand != "" {
							// emulate the user replying ...
							cmd := api.ParseCommand(0, "iam", userCommand)
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

			command := api.ParseCommand(0, "iam", tt.cmd)
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

func TestPosition_Track(t *testing.T) {

}
