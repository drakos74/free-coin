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
		"only-Value": {
			transform: func(i int) string {
				return "1"
			},
			predictions: map[string]Prediction{
				"1": {
					Value:       "1",
					Probability: 1,
					Options:     1,
					Sample:      98,
				},
			},
			sizes: []int{1},
		},
		"single-Value": {
			transform: func(i int) string {
				return "1"
			},
			predictions: map[string]Prediction{
				"1:1": {
					Value:       "1",
					Probability: 1,
					Options:     1,
					Sample:      97,
				},
			},
			sizes: []int{2},
		},
		"dual-Value": {
			transform: func(i int) string {
				if i%2 == 0 {
					return "1"
				}
				return "2"
			},
			predictions: map[string]Prediction{
				"1:2": {
					Value:       "1",
					Probability: 1,
					Options:     1,
					Sample:      49,
				},
				"2:1": {
					Value:       "2",
					Probability: 1,
					Options:     1,
					// NOTE : it s one less, because the first instance is the other option.
					Sample: 48,
				},
			},
			sizes: []int{2},
		},
		"sequence-Value": {
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
					Value:       "1",
					Probability: 0.5,
					Options:     2,
					Sample:      32,
				},
				// NOTE : this never comes up
				//"1:2:2": {},
				"1:1:1": {
					Value:       "2",
					Probability: 1,
					Options:     1,
					Sample:      15,
				},
				"1:1:2": {
					Value:       "1",
					Probability: 1,
					Options:     1,
					Sample:      15,
				},
				"2:1:1": {
					Value:       "1",
					Probability: 1,
					Options:     1,
					Sample:      16,
				},
				"2:1:2": {
					Value:       "1",
					Probability: 1,
					Options:     1,
					Sample:      15,
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
				assert.Equal(t, prediction.Sample, pp.Sample, fmt.Sprintf("wrong Sample for key [%s]", k))
				assert.Equal(t, prediction.Probability, pp.Probability, fmt.Sprintf("wrong Probability for key [%s]", k))
				assert.Equal(t, prediction.Options, pp.Options, fmt.Sprintf("wrong Options for key [%s]", k))
				assert.Equal(t, prediction.Value, pp.Value, fmt.Sprintf("wrong Value for key [%s]", k))
			}
		})
	}

}
