package ml

import (
	"fmt"

	"github.com/drakos74/free-coin/internal/emoji"

	"github.com/drakos74/free-coin/internal/algo/processor"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"

	"github.com/drakos74/free-coin/internal/algo/processor/ml/net"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	json_storage "github.com/drakos74/free-coin/internal/storage/file/json"
)

const (
	realityThreshold    = 1
	predictionThreshold = 0.9
)

func ReProcessor() (proc api.Processor, err error) {

	performance := make(map[model.Key]map[mlmodel.Detail]Performance)
	prediction := make(map[model.Key]map[mlmodel.Detail][][]float64)

	config := CoinConfig(map[model.Coin]mlmodel.ConfigSegment{
		model.BTC: func(coin model.Coin) func(cfg mlmodel.SegmentConfig) mlmodel.SegmentConfig {
			return func(cfg mlmodel.SegmentConfig) mlmodel.SegmentConfig {
				cfg[ConfigKey(coin, 15)] = defaultConfig(true)
				return cfg
			}
		},
	})

	config.Buffer.History = false
	shard := json_storage.BlobShard("train-ml")
	col, err := NewCollector(shard, *config, CollectStats, Trend)
	if err != nil {
		return nil, err
	}

	var networkConstructor = net.BaseNetworkConstructor(8, 3)
	networks := make(map[model.Key]*net.BaseNetwork)

	// process the collector vectors for sophisticated analysis
	go func(col *Collector) {
		num := 0
		for vv := range col.vectors {
			num++
			t := vv.Meta.Tick.Time
			p := vv.NewIn[len(vv.NewIn)-1]
			coin := string(vv.Meta.Key.Coin)
			duration := vv.Meta.Key.Duration.String()
			metrics.Observer.IncrementEvents(coin, duration, "event", Name)
			configSegments := config.GetSegments(vv.Meta.Key.Coin, vv.Meta.Key.Duration)

			if num%100 == 0 {
				fmt.Printf("%v | %f | performance \n", t, p)
				for _, dp := range performance {
					for d, perf := range dp {
						fmt.Printf("%s = %+v\n", d.ToString(), perf)
					}
				}
			}

			for key, segments := range configSegments {

				if _, ok := prediction[key]; !ok {
					prediction[key] = make(map[mlmodel.Detail][][]float64)
				}

				if _, ok := performance[key]; !ok {
					performance[key] = make(map[mlmodel.Detail]Performance)
				}

				for detail, p := range prediction[key] {
					if _, ok := performance[key][detail]; !ok {
						performance[key][detail] = Performance{
							num:            0,
							total:          0,
							falsePositives: make(map[string]int),
						}
					}

					perf := performance[key][detail]
					// compare new vector with previous prediction
					reality := 0.0
					if vv.PrevOut[0] > 0.5 {
						reality = 1
					} else if vv.PrevOut[0] < -0.5 {
						reality = -1
					}
					pred := p[0][0]

					result := reality * pred
					if result < 0 {
						// totally wrong
						perf.falsePositives[emoji.Loss] += 1
					} else if result > 0 {
						// got the money
						perf.falsePositives[emoji.DotFire] += 1
					} else if reality == 0.0 && pred != 0 {
						// not quite
						perf.falsePositives[emoji.DotWater] += 1
					} else if pred == 0.0 && reality != 0 {
						// missed opportunity
						perf.falsePositives[emoji.EclipseFace] += 1
					} else {
						// neutral
						perf.falsePositives[emoji.SunFace] += 1
					}
					if pred != 0 {
						perf.total += 1
					}
					perf.num += 1
					performance[key][detail] = perf
				}

				// do our training here ...
				if key.Match(vv.Meta.Key.Coin) {
					metrics.Observer.IncrementEvents(coin, duration, "train", Name)
					if _, ok := networks[key]; !ok {
						// create the network for this coin set up for the first encounter
						networks[key] = networkConstructor(key, segments)
					}
					network := networks[key]
					out, done, err := network.Push(key, vv)
					//fmt.Printf("out = %+v\n", out)
					if err != nil {
						panic(fmt.Errorf("error during network training: %+v", err))
						return
					}
					if done {
						prediction[key] = out
					}
				}
			}
		}
	}(col)

	return processor.ProcessBufferedWithClose("reprocessor", config.Segment.Interval, false, func(trade *model.TradeSignal) error {
		col.push(trade)
		return nil
	}, nil), nil
}
