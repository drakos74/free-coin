package trade

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/drakos74/free-coin/internal/model"
)

func TestTradeStrategy(t *testing.T) {

	type test struct {
		vv       [][]string
		strategy TradingStrategy
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
			strategy: numericStrategy,
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
			strategy: numericStrategy,
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
			strategy: numericStrategy,
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
