package processor

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/drakos74/free-coin/internal/api"

	"github.com/drakos74/free-coin/internal/model"
)

func TestTrader_Gather(t *testing.T) {

	type test struct {
		transform func(i int) float64
		msgCount  int
		config    openConfig
	}

	tests := map[string]test{
		"sin": {
			transform: func(i int) float64 {
				return math.Sin(float64(i/10) * 40000)
			},
			msgCount: 13,
			config: openConfig{
				coin:                 model.BTC,
				sampleThreshold:      1,
				probabilityThreshold: 0.9,
				volume:               0.1,
			},
		},
		"inc": {
			transform: func(i int) float64 {
				return 40000 + 100*float64(i)
			},
			msgCount: 35,
		},
		"dec": {
			transform: func(i int) float64 {
				return 40000 - 100*float64(i)
			},
			msgCount: 35,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			in := make(chan *model.Trade)
			st := make(chan *model.Trade)
			client := newMockClient()

			signals := make(chan api.Signal)
			_, statsMessages := run(client, in, st, func(client api.Exchange, user api.User) api.Processor {
				return MultiStats(client, user, signals)
			})
			out := make(chan *model.Trade)
			cmds, tradeMessages := run(client, st, out, func(client api.Exchange, user api.User) api.Processor {
				return Trade(client, user, signals)
			})

			// send the config to the processor
			cmds <- api.Command{
				ID:      1,
				User:    "",
				Content: fmt.Sprintf("?t BTC 10 %f %d", tt.config.probabilityThreshold, tt.config.sampleThreshold),
			}

			wg := new(sync.WaitGroup)
			wg.Add(tt.msgCount)
			go logMessages("stats", nil, statsMessages)
			go logMessages("trade", wg, tradeMessages)

			num := 1000
			wg.Add(num)
			go func() {
				start := time.Now()
				for i := 0; i < num; i++ {
					trade := newTrade(model.BTC, tt.transform(i), 1, model.Buy, start.Add(time.Duration(i*15)*time.Second))
					// enable trade to publish messages
					trade.Live = true
					in <- trade
				}
			}()

			go func() {
				for range out {

				}
			}()

			wg.Wait()
		})
	}

}

func TestTradeStrategy(t *testing.T) {

	type test struct {
		vv       [][]string
		strategy tradingStrategy
		ttyp     model.Type
	}

	tests := map[string]test{
		"buy": {
			vv: [][]string{
				{"+0", "+1"},
				{"+0", "+1", "+2"},
				{"+0", "+2"},
				{"+0", "+2", "+5"},
				{"+0", "+3"},
				{"+0", "+3", "+3"},
				{"+0", "+4"},
				{"+1", "+1"},
				{"+1", "+1", "+4"},
				{"+1", "+2"},
				{"+1", "+2", "+2"},
				{"+1", "+3"},
				{"+1", "+3", "+0"},
				{"+1", "+4"},
				{"+2", "+0"},
				{"+2", "+1"},
				{"+2", "+1", "+1"},
				{"+2", "+2"},
				{"+3", "+0"},
			},
			strategy: simpleStrategy,
			ttyp:     1,
		},
		"sell": {
			vv: [][]string{
				{"-0", "-1"},
				{"-0", "-1", "-2"},
				{"-0", "-2"},
				{"-0", "-2", "-5"},
				{"-0", "-3"},
				{"-0", "-3", "-3"},
				{"-0", "-4"},
				{"-1", "-1"},
				{"-1", "-1", "-4"},
				{"-1", "-2"},
				{"-1", "-2", "-2"},
				{"-1", "-3"},
				{"-1", "-3", "-0"},
				{"-1", "-4"},
				{"-2", "-0"},
				{"-2", "-1"},
				{"-2", "-1", "-1"},
				{"-2", "-2"},
				{"-3", "-0"},
			},
			strategy: simpleStrategy,
			ttyp:     2,
		},
		"no-action": {
			vv: [][]string{
				{"+0", "-0"},
				{"+1", "-1"},
				{"+2", "-2"},
				{"+3", "+1"},
				{"+2", "+1", "+2"},
				{"+0", "+3", "+4"},
				{"+1", "+1", "+5"},
				{"-3", "-1"},
				{"-2", "-1", "-2"},
				{"-0", "-3", "-4"},
				{"-1", "-1", "-5"},
			},
			strategy: simpleStrategy,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := newOpenConfig(model.BTC, 0.1)

			for _, v := range tt.vv {
				ttyp := cfg.contains(v)
				assert.Equal(t, tt.ttyp, ttyp, fmt.Sprintf("failed %v for %v", tt.ttyp, v))
			}

		})
	}

}
