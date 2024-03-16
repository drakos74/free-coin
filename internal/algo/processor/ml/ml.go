package ml

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/buffer"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"

	"github.com/drakos74/free-coin/internal/emoji"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/algo/processor/ml/net"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

const (
	Name = "ml-network"
)

type Tracker struct {
	Buffer      *buffer.MultiBuffer
	Performance map[mlmodel.Detail]Performance
	Prediction  map[mlmodel.Detail][][]float64
}

type Performance struct {
	num            int
	total          int
	falsePositives map[string]int
}

func (p Performance) String() string {
	buffer := new(strings.Builder)

	buffer.WriteString(fmt.Sprintf("%d / %d | %.2f : %.2f",
		p.total, p.num, p.Value(false), p.Value(true)))
	//for s, v := range p.falsePositives {
	//	buffer.WriteString(fmt.Sprintf("%s : %d\n", s, v))
	//}
	return buffer.String()
}

func (p Performance) Value(lazy bool) float64 {
	denom := p.falsePositives[emoji.Loss]
	if lazy {
		denom += p.falsePositives[emoji.DotWater]
	}
	if denom == 0 {
		return float64(p.falsePositives[emoji.DotFire])
	}
	return float64(p.falsePositives[emoji.DotFire]) / float64(denom)
}

// Processor is the position processor main routine.
func Processor(index api.Index, shard storage.Shard, strategy *processor.Strategy) func(u api.User, e api.Exchange) api.Processor {
	config := strategy.Config()
	col, err := NewCollector(shard, config, Trend, Trend)
	// make sure we don't break the pipeline
	if err != nil {
		log.Error().Err(err).Str("processor", Name).Msg("could not init processor")
		return func(u api.User, e api.Exchange) api.Processor {
			return processor.Void(Name)
		}
	}

	var networkConstructor = net.BaseNetworkConstructor(8, 3)
	networks := make(map[model.Key]*net.BaseNetwork)

	tracker := make(map[model.Key]*Tracker)

	return func(u api.User, e api.Exchange) api.Processor {
		// init the user interactions
		go trackUserActions(index, u, strategy, tracker)
		u.Send(index, api.NewMessage(fmt.Sprintf("%s starting processor ... %s", Name, formatConfig(config))), nil)

		numEvents := 0
		// process the collector vectors for sophisticated analysis
		go func(col *Collector) {
			for vv := range col.vectors {
				coin := string(vv.Meta.Key.Coin)
				duration := vv.Meta.Key.Duration.String()
				metrics.Observer.IncrementEvents(coin, duration, "collector", Name)
				cfg := strategy.Config()
				configSegments := cfg.GetSegments(vv.Meta.Key.Coin, vv.Meta.Key.Duration)

				numEvents++
				t := vv.Meta.Tick.Time
				p := vv.NewIn[len(vv.NewIn)-1]
				for key, segments := range configSegments {

					// process only if we have it enabled
					if !strategy.IsEnabledML(key) {
						return
					}

					if _, ok := tracker[key]; !ok {
						tracker[key] = &Tracker{
							// track the latest data
							Buffer:      buffer.NewMultiBuffer(5),
							Performance: make(map[mlmodel.Detail]Performance),
							Prediction:  make(map[mlmodel.Detail][][]float64),
						}
					}

					if key.Match(vv.Meta.Key.Coin) {
						// track the trend and price ...
						tracker[key].Buffer.Push(vv.NewIn[0], vv.NewIn[len(vv.NewIn)-1])
						// track the performance
						for detail, prediction := range tracker[key].Prediction {
							if _, ok := tracker[key].Performance[detail]; !ok {
								tracker[key].Performance[detail] = Performance{
									num:            0,
									total:          0,
									falsePositives: make(map[string]int),
								}
							}

							perf := tracker[key].Performance[detail]
							// compare new vector with previous prediction
							reality := 0.0
							if vv.PrevOut[0] > 0.5 {
								reality = 1
							} else if vv.PrevOut[0] < -0.5 {
								reality = -1
							}
							pred := prediction[0][0]

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
							tracker[key].Performance[detail] = perf
						}
						// do our training here ...
						metrics.Observer.IncrementEvents(coin, duration, "train", Name)
						if _, ok := networks[key]; !ok {
							// create the network for this coin set up for the first encounter
							networks[key] = networkConstructor(key, segments)
						}
						network := networks[key]
						out, done, err := network.Push(key, vv)
						if hasTrigger(out) {
							u.Send(index,
								api.NewMessage(formatOutPredictions(t, key, p, out, tracker[key].Performance)).
									AddLine(formatRecentData(tracker[key].Buffer.Get())),
								nil)
						}
						//fmt.Printf("out = %+v\n", out)
						if err != nil {
							panic(fmt.Errorf("error during network training: %+v", err))
							return
						}
						if done {
							for d, o := range out {
								tracker[key].Prediction[d] = o
							}
						}
					}
				}
			}
		}(col)

		return processor.ProcessBufferedWithClose(Name, config.Segment.Interval, true, func(tradeSignal *model.TradeSignal) error {
			// TODO : make this generic part of the processor `ProcessBufferedWithClose`
			coin := string(tradeSignal.Coin)
			f, _ := strconv.ParseFloat(tradeSignal.Meta.Time.Format("20060102.1504"), 64)
			start := time.Now()
			metrics.Observer.NoteLag(f, coin, Name, "batch")
			metrics.Observer.IncrementTrades(coin, Name, "batch")
			// push to the collector for further analysis
			col.push(tradeSignal)
			// track the collector processing duration
			// TODO : make this generic part of the processor `ProcessBufferedWithClose`
			duration := time.Now().Sub(start).Seconds()
			metrics.Observer.TrackDuration(duration, coin, Name, "process")
			return nil
		}, func() {
			// something terrible has happened here ...
		}, processor.Deriv())
	}
}

func hasTrigger(out map[mlmodel.Detail][][]float64) bool {
	for _, vv := range out {
		for _, v := range vv {
			if v[0] > 0 || v[0] < 0 {
				return true
			}
		}
	}
	return false
}
