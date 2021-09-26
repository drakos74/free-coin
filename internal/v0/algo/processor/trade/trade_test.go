package trade

import (
	"fmt"
	"testing"

	"github.com/drakos74/free-coin/internal/storage"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/stretchr/testify/assert"
)

// TODO : fix these tests
func TestTradeStrategy(t *testing.T) {

	type test struct {
		vv       []buffer.Sequence
		strategy processor.Strategy
		ttyp     model.Type
		action   bool
	}

	tests := map[string]test{
		"buy": {
			action: true,
			vv: []buffer.Sequence{
				"+0:+1",
				"+0:+1:+2",
				"+0:+2",
				"+0:+2:+5",
				"+0:+3",
				"+0:+3:+3",
				"+0:+4",
				"+1:+1",
				"+1:+1:+4",
				"+1:+2",
				"+1:+2:+2",
				"+1:+3",
				"+1:+3:+0",
				"+1:+4",
				"+2:+0",
				"+2:+1",
				"+2:+1:+1",
				"+2:+2",
				"+3:+0",
			},
			strategy: processor.Strategy{
				Name:      processor.NumericStrategy,
				Threshold: 10,
			},
			ttyp: 1,
		},
		"sell": {
			action: true,
			vv: []buffer.Sequence{
				"-0:-1",
				"-0:-1:-2",
				"-0:-2",
				"-0:-2:-5",
				"-0:-3",
				"-0:-3:-3",
				"-0:-4",
				"-1:-1",
				"-1:-1:-4",
				"-1:-2",
				"-1:-2:-2",
				"-1:-3",
				"-1:-3:-0",
				"-1:-4",
				"-2:-0",
				"-2:-1",
				"-2:-1:-1",
				"-2:-2",
				"-3:-0",
			},
			strategy: processor.Strategy{
				Name:      processor.NumericStrategy,
				Threshold: 10,
			},
			ttyp: 2,
		},
		"no-action": {
			action: false,
			vv: []buffer.Sequence{
				"+0:-0",
				"+1:-1",
				"+2:-2",
				"+3:+1",
				"+2:+1:+2",
				"+0:+3:+4",
				"+1:+1:+5",
				"-3:-1",
				"-2:-1:-2",
				"-0:-3:-4",
				"-1:-1:-5",
			},
			strategy: processor.Strategy{
				Name:      processor.NumericStrategy,
				Threshold: 10,
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {

			for _, v := range tt.vv {

				var pList buffer.PredictionList = make([]*buffer.Prediction, len(tt.vv))

				for _, vv := range tt.vv {
					pList = append(pList, &buffer.Prediction{
						Value: vv,
					})
				}

				prediction := buffer.Predictions{
					Values: pList,
					Sample: 1,
				}
				tr := &trader{
					registry: storage.NewVoidRegistry(),
				}
				executor := tr.getStrategy(tt.strategy.Name)
				values, probability, confidence, ttype := executor(SignalEvent{}, prediction, tt.strategy)
				println(fmt.Sprintf("values = %+v", values))
				println(fmt.Sprintf("probability = %+v", probability))
				println(fmt.Sprintf("confidence = %+v", confidence))
				assert.Equal(t, tt.action, ttype != model.NoType)
				assert.Equal(t, tt.ttyp, ttype, fmt.Sprintf("failed %v for %v", tt.ttyp, v))
			}

		})
	}

}
