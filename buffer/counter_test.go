package buffer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounter_Add(t *testing.T) {

	type test struct {
		transform   func(i int) string
		predictions map[string]Prediction
		sizes       []int
	}

	tests := map[string]test{
		"only-value": {
			transform: func(i int) string {
				return "1"
			},
			predictions: map[string]Prediction{
				"1": {
					value:       "1",
					probability: 1,
					options:     1,
					sample:      98,
				},
			},
			sizes: []int{1},
		},
		"single-value": {
			transform: func(i int) string {
				return "1"
			},
			predictions: map[string]Prediction{
				"1:1": {
					value:       "1",
					probability: 1,
					options:     1,
					sample:      97,
				},
			},
			sizes: []int{2},
		},
		"dual-value": {
			transform: func(i int) string {
				if i%2 == 0 {
					return "1"
				}
				return "2"
			},
			predictions: map[string]Prediction{
				"1:2": {
					value:       "1",
					probability: 1,
					options:     1,
					sample:      49,
				},
				"2:1": {
					value:       "2",
					probability: 1,
					options:     1,
					// NOTE : it s one less, because the first instance is the other option.
					sample: 48,
				},
			},
			sizes: []int{2},
		},
		"sequence-value": {
			transform: func(i int) string {
				if i%2 == 0 {
					return "1"
				} else if i%3 == 0 {
					return "1"
				}
				return "2"
			},
			predictions: map[string]Prediction{
				"1:2:1": {
					value:       "1",
					probability: 0.5,
					options:     2,
					sample:      32,
				},
				// NOTE : this never comes up
				//"1:2:2": {},
				"1:1:1": {
					value:       "2",
					probability: 1,
					options:     1,
					sample:      15,
				},
				"1:1:2": {
					value:       "1",
					probability: 1,
					options:     1,
					sample:      15,
				},
				"2:1:1": {
					value:       "1",
					probability: 1,
					options:     1,
					sample:      16,
				},
				"2:1:2": {
					value:       "1",
					probability: 1,
					options:     1,
					sample:      15,
				},
			},
			sizes: []int{3},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := NewCounter(tt.sizes...)
			p := make(map[string]Prediction)
			for i := 0; i < 100; i++ {
				// we keep track of the last prediction to assert on all possible outcomes
				pp := c.Add(tt.transform(i))
				for kp, vp := range pp {
					p[kp] = vp
				}
			}

			assert.Equal(t, len(p), len(tt.predictions))

			for k, prediction := range tt.predictions {
				pp, ok := p[k]
				assert.True(t, ok, fmt.Sprintf("missing key [%s]", k))
				assert.Equal(t, prediction.sample, pp.sample, fmt.Sprintf("wrong sample for key [%s]", k))
				assert.Equal(t, prediction.probability, pp.probability, fmt.Sprintf("wrong probability for key [%s]", k))
				assert.Equal(t, prediction.options, pp.options, fmt.Sprintf("wrong options for key [%s]", k))
				assert.Equal(t, prediction.value, pp.value, fmt.Sprintf("wrong value for key [%s]", k))
			}
		})
	}

}
