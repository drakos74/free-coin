package net

import (
	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/model"
)

type HMM struct {
	SingleNetwork
	*buffer.HMM
}

func ConstructHMM() ConstructNetwork {
	return func(cfg mlmodel.Model) Network {
		return NewHMM(cfg,
			buffer.HMMConfig{
				LookBack:  3,
				LookAhead: 1,
			},
			buffer.HMMConfig{
				LookBack:  2,
				LookAhead: 2,
			},
		)
	}
}

func NewHMM(cfg mlmodel.Model, config ...buffer.HMMConfig) *HMM {
	return &HMM{
		SingleNetwork: NewSingleNetwork(cfg),
		HMM:           buffer.NewMultiHMM(config...),
	}
}

func toEmoji(v []float64) string {
	if len(v) >= 3 {
		if v[0] == 1 {
			return emoji.DotFire
		} else if v[2] == 1 {
			return emoji.DotWater
		}
	}
	return emoji.DotSnow
}

func (hmm *HMM) Train(ds *Dataset) ModelResult {
	vector := ds.Vectors[len(ds.Vectors)-1]
	v := toEmoji(vector.PrevOut)
	predictions, status := hmm.Add(v, "train")
	//fmt.Printf("predictions = %+v\n", predictions)
	//fmt.Printf("status = %+v\n", status)
	result := ModelResult{}
	if int(status.Count) > hmm.config.BufferSize {
		for _, p := range predictions {
			for _, prediction := range p.Values {
				if !result.OK && prediction.Probability > hmm.config.PrecisionThreshold {
					t := model.NoType
					for _, value := range prediction.Value.Values() {
						if value == emoji.DotFire && (t == model.Buy || t == model.NoType) {
							t = model.Buy
						} else if value == emoji.DotWater && (t == model.Sell || t == model.NoType) {
							t = model.Sell
						}
					}
					if t != model.NoType {
						result.Type = t
						result.OK = true
					}
				}
			}
		}
	}
	return result
}
