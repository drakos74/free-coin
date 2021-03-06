package buffer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCounter_Add(t *testing.T) {

	type test struct {
		transform   func(i int) string
		predictions []Predictions
		configs     []HMMConfig
	}

	tests := map[string]test{
		"only-value": {
			transform: func(i int) string {
				return "1"
			},
			predictions: []Predictions{
				{
					Key:    "1",
					Sample: 98,
					Groups: 1,
					Values: PredictionList{{
						Value:       "1",
						Probability: 1,
					}},
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
			predictions: []Predictions{
				{
					Key:    "1:1",
					Sample: 97,
					Groups: 1,
					Values: PredictionList{{
						Value:       "1",
						Probability: 1,
					}},
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
			predictions: []Predictions{
				{
					Key:    "1:2",
					Sample: 49,
					Groups: 1,
					Values: PredictionList{{
						Value:       "1",
						Probability: 1,
					}},
				},
				{
					Key:    "2:1",
					Sample: 48,
					Groups: 1,
					Values: PredictionList{{
						Value:       "2",
						Probability: 1,
					}},
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
			predictions: []Predictions{
				{
					Key:    "1:2",
					Sample: 48,
					Groups: 1,
					Values: PredictionList{{
						Value:       "1:2",
						Probability: 1,
					}},
				},
				{
					Key:    "2:1",
					Sample: 47,
					Groups: 1,
					Values: PredictionList{{
						Value:       "2:1",
						Probability: 1,
					}},
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
			predictions: []Predictions{
				// NOTE : this never comes up
				// Key:    "1:2:2",
				{
					Key:    "1:2:1",
					Sample: 32,
					Groups: 1,
					Values: PredictionList{
						{
							Value:       "1",
							Probability: 0.5,
						},
						{
							Value:       "2",
							Probability: 0.5,
						},
					},
				},
				{
					Key:    "1:1:1",
					Sample: 15,
					Groups: 1,
					Values: PredictionList{{
						Value:       "2",
						Probability: 1,
					}},
				},
				{
					Key:    "1:1:2",
					Sample: 15,
					Groups: 1,
					Values: PredictionList{{
						Value:       "1",
						Probability: 1,
					}},
				},
				{
					Key:    "2:1:1",
					Sample: 16,
					Groups: 2,
					Values: PredictionList{{
						Value:       "1",
						Probability: 1,
					}},
				},
				{
					Key:    "2:1:2",
					Sample: 15,
					Groups: 2,
					Values: PredictionList{{
						Value:       "1",
						Probability: 1,
					}},
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
			predictions: []Predictions{
				// NOTE : this never comes up
				// Key:    "1:2:2",
				{
					Key:    "1:2:1",
					Sample: 31,
					Groups: 2,
					Values: PredictionList{
						{
							Value:       "1:1",
							Probability: 0.52,
						},
						{
							Value:       "2:1",
							Probability: 0.48,
						}},
				},
				{
					Key:    "1:1:1",
					Sample: 15,
					Groups: 2,
					Values: PredictionList{{
						Value:       "2:1",
						Probability: 1,
					}},
				},
				{
					Key:    "1:1:2",
					Sample: 15,
					Groups: 1,
					Values: PredictionList{
						{
							Value:       "1:2",
							Probability: 1,
						}},
				},
				{
					Key:    "2:1:1",
					Sample: 16,
					Groups: 1,
					Values: PredictionList{{
						Value:       "1:2",
						Probability: 1,
					}},
				},
				{
					Key:    "2:1:2",
					Sample: 15,
					Groups: 1,
					Values: PredictionList{{
						Value:       "1:1",
						Probability: 1,
					}},
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
			predictions: []Predictions{
				{
					Key:    "1",
					Sample: 65,
					Groups: 2,
					Values: PredictionList{
						{
							Value:       "2",
							Probability: 0.51,
						},
						{
							Value:       "1",
							Probability: 0.49,
						},
					},
				},
				{
					Key:    "2",
					Sample: 32,
					Groups: 2,
					Values: PredictionList{
						{
							Value:       "1",
							Probability: 1,
						},
					},
				},
				{
					Key:    "1:1",
					Sample: 32,
					Groups: 2,
					Values: PredictionList{
						{
							Value:       "2",
							Probability: 0.5,
						},
						{
							Value:       "1",
							Probability: 0.5,
						},
					},
				},
				{
					Key:    "2:1",
					Sample: 32,
					Groups: 1,
					Values: PredictionList{
						{
							Value:       "1",
							Probability: 0.5,
						},
						{
							Value:       "2",
							Probability: 0.5,
						},
					},
				},
				{
					Key:    "1:2",
					Sample: 32,
					Groups: 2,
					Values: PredictionList{{
						Value:       "1",
						Probability: 1,
					}},
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
			p := make(map[Sequence]Predictions)
			for i := 0; i < 100; i++ {
				// we keep track of the last prediction to assert on all possible outcomes
				s := tt.transform(i)
				// TODO : assert the Status
				pp, _ := c.Add(s, "label")

				// track the last j configs
				vv := make(map[Sequence]struct{})
				for _, j := range tt.configs {
					index := make([]string, j.LookBack)
					l := 0
					for k := j.LookBack - 1; k >= 0; k-- {
						index[k] = tt.transform(i - l)
						l++
					}
					vv[NewSequence(index)] = struct{}{}
				}

				for kp, vp := range pp {
					p[kp] = vp
					_, ok := vv[kp]
					// make sure our predictions relate to the latest slices
					assert.True(t, ok)
				}
			}

			for _, prediction := range tt.predictions {
				pp, ok := p[prediction.Key]
				assert.True(t, ok, fmt.Sprintf("missing key [%s]", prediction.Key))
				assert.Equal(t, prediction.Sample, pp.Sample, fmt.Sprintf("wrong Sample for key [%s]", prediction.Key))
				assert.Equal(t, prediction.Groups, pp.Groups, fmt.Sprintf("wrong Groups for key [%s]", prediction.Key))
				assert.Equal(t, len(prediction.Values), len(pp.Values), fmt.Sprintf("wrong number of values for key [%s]", prediction.Key))
				for i, ppValue := range prediction.Values {
					var found = true
					for _, v := range pp.Values {
						if v.Value == ppValue.Value {
							assert.Equal(t, fmt.Sprintf("%.2f", v.Probability), fmt.Sprintf("%.2f", ppValue.Probability), fmt.Sprintf("wrong Probability for key [%s] at index %d", prediction.Key, i))
						}
					}
					assert.True(t, found, fmt.Sprintf("wrong Value for key [%s] at index %d", prediction.Key, i))
				}
				delete(p, prediction.Key)
			}

			// we should have matched ALL predictions by now
			assert.Equal(t, 0, len(p), fmt.Sprintf("%+v", p))

		})
	}

}
