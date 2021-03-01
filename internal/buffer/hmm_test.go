package buffer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounter_Add(t *testing.T) {

	type test struct {
		transform   func(i int) string
		predictions map[string]Prediction
		configs     []HMMConfig
	}

	tests := map[string]test{
		"only-value": {
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
			configs: []HMMConfig{{
				LookBack:  1,
				LookAhead: 1,
			}},
		},
		"single-value": {
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
			configs: []HMMConfig{{
				LookBack:  2,
				LookAhead: 1,
			}},
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
			configs: []HMMConfig{{
				LookBack:  2,
				LookAhead: 1,
			}},
		},
		"dual-target-value": {
			transform: func(i int) string {
				if i%2 == 0 {
					return "1"
				}
				return "2"
			},
			predictions: map[string]Prediction{
				"1:2": {
					Value:       "1:2",
					Probability: 1,
					Options:     1,
					Sample:      48,
				},
				"2:1": {
					Value:       "2:1",
					Probability: 1,
					Options:     1,
					// NOTE : it s one less, because the first instance is the other option.
					Sample: 47,
				},
			},
			configs: []HMMConfig{{
				LookBack:  2,
				LookAhead: 2,
			}},
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
			configs: []HMMConfig{
				{
					LookBack:  3,
					LookAhead: 1,
				},
			},
		},
		"sequence-target-value": {
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
					Value:       "1:1",
					Probability: 0.52,
					Options:     2,
					Sample:      31,
				},
				// NOTE : this never comes up
				//"1:2:2": {},
				"1:1:1": {
					Value:       "2:1",
					Probability: 1,
					Options:     1,
					Sample:      15,
				},
				"1:1:2": {
					Value:       "1:2",
					Probability: 1,
					Options:     1,
					Sample:      15,
				},
				"2:1:1": {
					Value:       "1:2",
					Probability: 1,
					Options:     1,
					Sample:      16,
				},
				"2:1:2": {
					Value:       "1:1",
					Probability: 1,
					Options:     1,
					Sample:      15,
				},
			},
			configs: []HMMConfig{
				{
					LookBack:  3,
					LookAhead: 2,
				},
			},
		},
		"multi-length": {
			transform: func(i int) string {
				if i%2 == 0 {
					return "1"
				} else if i%3 == 0 {
					return "1"
				}
				return "2"
			},
			predictions: map[string]Prediction{
				"1": {
					Value:       "2",
					Probability: 0.51,
					Options:     2,
					Sample:      65,
				},
				"2": {
					Value:       "1",
					Probability: 1,
					Options:     1,
					Sample:      32,
				},
				// NOTE : this never comes up
				//"1:2:2": {},
				"1:1": {
					Value:       "1",
					Probability: 0.5,
					Options:     2,
					Sample:      32,
				},
				"2:1": {
					Value:       "1",
					Probability: 0.5,
					Options:     2,
					Sample:      32,
				},
				"1:2": {
					Value:       "1",
					Probability: 1,
					Options:     1,
					Sample:      32,
				},
			},
			configs: []HMMConfig{
				{
					LookBack:  1,
					LookAhead: 1,
				},
				{
					LookBack:  2,
					LookAhead: 1,
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := NewMultiHMM(tt.configs...)
			p := make(map[string]Prediction)
			for i := 0; i < 100; i++ {
				// we keep track of the last prediction to assert on all possible outcomes
				s := tt.transform(i)
				// TODO : assert the status
				pp, _ := c.Add(s, "label")

				// track the last j configs
				vv := make(map[string]struct{})
				for _, j := range tt.configs {
					index := make([]string, j.LookBack)
					l := 0
					for k := j.LookBack - 1; k >= 0; k-- {
						index[k] = tt.transform(i - l)
						l++
					}
					vv[strings.Join(index, ":")] = struct{}{}
				}

				for kp, vp := range pp {
					p[kp] = vp
					_, ok := vv[kp]
					// make sure our predictions relate to the latest slices
					assert.True(t, ok)
				}
			}

			assert.Equal(t, len(tt.predictions), len(p))

			for k, prediction := range tt.predictions {
				pp, ok := p[k]
				assert.True(t, ok, fmt.Sprintf("missing key [%s]", k))
				assert.Equal(t, prediction.Sample, pp.Sample, fmt.Sprintf("wrong Sample for key [%s]", k))
				assert.Equal(t, fmt.Sprintf("%.2f", prediction.Probability), fmt.Sprintf("%.2f", pp.Probability), fmt.Sprintf("wrong Probability for key [%s]", k))
				assert.Equal(t, prediction.Options, pp.Options, fmt.Sprintf("wrong Options for key [%s]", k))
				assert.Equal(t, prediction.Value, pp.Value, fmt.Sprintf("wrong Value for key [%s]", k))
			}
		})
	}

}
