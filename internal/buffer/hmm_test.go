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
		eventCount  int
	}

	tests := map[string]test{
		"only-value": {
			eventCount: 100,
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
						EMP:         0.99,
					}},
				},
			},
			configs: []HMMConfig{{
				LookBack:  1,
				LookAhead: 1,
			}},
		},
		"single-value": {
			eventCount: 100,
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
						EMP:         0.99,
					}},
				},
			},
			configs: []HMMConfig{{
				LookBack:  2,
				LookAhead: 1,
			}},
		},
		"dual-value": {
			eventCount: 100,
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
						EMP:         0.98,
					}},
				},
				{
					Key:    "2:1",
					Sample: 48,
					Groups: 1,
					Values: PredictionList{{
						Value:       "2",
						Probability: 1,
						EMP:         0.98,
					}},
				},
			},
			configs: []HMMConfig{{
				LookBack:  2,
				LookAhead: 1,
			}},
		},
		"dual-target-value": {
			eventCount: 100,
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
						EMP:         0.98,
					}},
				},
				{
					Key:    "2:1",
					Sample: 47,
					Groups: 1,
					Values: PredictionList{{
						Value:       "2:1",
						Probability: 1,
						EMP:         0.98,
					}},
				},
			},
			configs: []HMMConfig{{
				LookBack:  2,
				LookAhead: 2,
			}},
		},
		"sequence-value": {
			eventCount: 100,
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
							EMP:         0.47,
						},
						{
							Value:       "2",
							Probability: 0.5,
							EMP:         0.5,
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
						EMP:         0.93,
					}},
				},
				{
					Key:    "1:1:2",
					Sample: 15,
					Groups: 1,
					Values: PredictionList{{
						Value:       "1",
						Probability: 1,
						EMP:         0.93,
					}},
				},
				{
					Key:    "2:1:1",
					Sample: 16,
					Groups: 2,
					Values: PredictionList{{
						Value:       "1",
						Probability: 1,
						EMP:         0.94,
					}},
				},
				{
					Key:    "2:1:2",
					Sample: 15,
					Groups: 2,
					Values: PredictionList{{
						Value:       "1",
						Probability: 1,
						EMP:         0.93,
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
			eventCount: 100,
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
							EMP:         0.5,
						},
						{
							Value:       "2:1",
							Probability: 0.48,
							EMP:         0.47,
						}},
				},
				{
					Key:    "1:1:1",
					Sample: 15,
					Groups: 2,
					Values: PredictionList{{
						Value:       "2:1",
						Probability: 1,
						EMP:         0.93,
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
							EMP:         0.93,
						}},
				},
				{
					Key:    "2:1:1",
					Sample: 16,
					Groups: 1,
					Values: PredictionList{{
						Value:       "1:2",
						Probability: 1,
						EMP:         0.94,
					}},
				},
				{
					Key:    "2:1:2",
					Sample: 15,
					Groups: 1,
					Values: PredictionList{{
						Value:       "1:1",
						Probability: 1,
						EMP:         0.93,
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
			eventCount: 100,
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
							EMP:         0.51,
						},
						{
							Value:       "1",
							Probability: 0.49,
							EMP:         0.48,
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
							EMP:         0.97,
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
							EMP:         0.5,
						},
						{
							Value:       "1",
							Probability: 0.5,
							EMP:         0.47,
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
							EMP:         0.47,
						},
						{
							Value:       "2",
							Probability: 0.5,
							EMP:         0.5,
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
						EMP:         0.97,
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
		"trend-capture-emp": {
			// increase the event count here, to make sure emp does not overflow over '1.0'
			eventCount: 10000,
			transform: func(i int) string {
				middle := 10000 / 2
				if i%4 == 0 {
					if i > middle {
						// this causes 2:3 -> 2
						return "2"
					} else {
						// this causes 2:3 -> 1
						return "1"
					}
				}
				return fmt.Sprintf("%d", i%4)
			},
			predictions: []Predictions{
				{
					Key:    "1:2",
					Sample: 2499,
					Groups: 1,
					Values: PredictionList{{
						Value:       "3",
						Probability: 1,
						EMP:         1,
					}},
				},
				{
					Key:    "2:1",
					Sample: 1248,
					Groups: 1,
					Values: PredictionList{{
						Value:       "2",
						Probability: 1,
						EMP:         1,
					}},
				},
				{
					Key:    "1:1",
					Sample: 1250,
					Groups: 1,
					Values: PredictionList{{
						Value:       "2",
						Probability: 1,
						EMP:         1,
					}},
				},
				{
					Key:    "2:3",
					Sample: 2499,
					Groups: 1,
					Values: PredictionList{
						{ // NOTE : emp is higher, as the last events are matching this pattern
							Value:       "2",
							Probability: 0.5,
							EMP:         0.75,
						},
						{ // NOTE : emp is lower, as the last events dont match this pattern
							Value:       "1",
							Probability: 0.5,
							EMP:         0.25,
						},
					},
				},
				{
					Key:    "3:1",
					Sample: 1249,
					Groups: 1,
					Values: PredictionList{{
						Value:       "1",
						Probability: 1,
						EMP:         1,
					}},
				},
				{
					Key:    "3:2",
					Sample: 1248,
					Groups: 2,
					Values: PredictionList{{
						Value:       "1",
						Probability: 1,
						EMP:         1,
					}},
				},
			},
			configs: []HMMConfig{{
				LookBack:  2,
				LookAhead: 1,
			}},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := NewMultiHMM(tt.configs...)
			p := make(map[Sequence]Predictions)
			for i := 0; i < tt.eventCount; i++ {
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

				for i, v := range pp.Values {
					var found bool
					for _, ppValue := range prediction.Values {
						if v.Value == ppValue.Value {
							found = true
							assert.Equal(t, fmt.Sprintf("%.2f", ppValue.Probability), fmt.Sprintf("%.2f", v.Probability), fmt.Sprintf("wrong Probability for key [%s] at value '%v'", prediction.Key, v.Value))
							assert.Equal(t, fmt.Sprintf("%.2f", ppValue.EMP), fmt.Sprintf("%.2f", v.EMP), fmt.Sprintf("wrong EMP for key [%s] at value '%v'", prediction.Key, v.Value))
						}
					}
					assert.True(t, found, fmt.Sprintf("not found value '%v' for key [%s] at index %d", v.Value, prediction.Key, i))
				}
				delete(p, prediction.Key)
			}

			// we should have matched ALL predictions by now
			assert.Equal(t, 0, len(p), fmt.Sprintf("%+v", p))

		})
	}

}
