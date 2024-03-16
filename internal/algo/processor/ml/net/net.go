package net

import (
	"fmt"
	"math"
	"reflect"
	"strings"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/buffer"
	coinmath "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/math/ml"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/go-ex-machina/xmath"
	"github.com/rs/zerolog/log"
)

func NewNetwork(s string, cfg mlmodel.Model) Network {
	switch s {
	case GRU_KEY:
		return NewGRU(cfg)
	case HMM_KEY:
		return NewHMM(cfg)
	case POLY_KEY:
		return NewPolynomial(cfg)
	case FOREST_KEY:
		return NewRandomForest(cfg)
	case NN_KEY:
		return NewNeuralNet(cfg)
	}
	panic(fmt.Sprintf("unknown network detail : %v", s))
	return nil
}

func quantify(y float64, gap float64) float64 {
	if y > gap {
		return 1
	} else if y < -1*gap {
		return -1
	} else {
		return 0
	}
}

func quantifyAll(v []float64, gap float64) []float64 {
	vv := make([]float64, len(v))
	for i := range v {
		vv[i] = quantify(v[i], gap)
	}
	return vv
}

func networkType(net Network) string {
	return reflect.TypeOf(net).Elem().String()
}

// Network is a generic network interface that receives a list of tensors and input an output
type Network interface {
	Train(x [][]float64, y [][]float64) (ml.Metadata, error)
	Predict(x [][]float64) ([][]float64, ml.Metadata, error)
	Loss(actual, predicted [][]float64) []float64
	Config() mlmodel.Model
	Load(key model.Key, detail mlmodel.Detail) error
	Save(key model.Key, detail mlmodel.Detail) error
}

// ConstructNetwork defines a network constructor func.
type ConstructNetwork func() (Network, mlmodel.Model)

// Stats defines generic network Stats.
type Stats struct {
	Accuracy   *buffer.Buffer
	Loss       *buffer.Buffer
	Prediction [][]float64
	Decisions  []int
}

func NewStats(s int) Stats {
	return Stats{
		Accuracy:  buffer.NewBuffer(s),
		Loss:      buffer.NewBuffer(s),
		Decisions: make([]int, s),
	}
}

// BaseNetwork represents the basic network flow logic
type BaseNetwork struct {
	set    DataSet
	net    map[mlmodel.Detail]Network
	config map[mlmodel.Detail]mlmodel.Model
	track  map[mlmodel.Detail]*Tracker
}

func BaseNetworkConstructor(in, out int) func(key model.Key, segments mlmodel.Segments) *BaseNetwork {
	return func(key model.Key, segments mlmodel.Segments) *BaseNetwork {
		gen := make([]ConstructNetwork, len(segments.Stats.Model))
		for i, segment := range segments.Stats.Model {
			gen[i] = func() (Network, mlmodel.Model) {
				return NewNetwork(segment.Detail.Type, segment), segment
			}
		}
		return NewBaseNetwork(key, in, out, gen...)
	}
}

func NewBaseNetwork(key model.Key, in, out int, gen ...ConstructNetwork) *BaseNetwork {

	trackers := make(map[mlmodel.Detail]*Tracker)
	networks := make(map[mlmodel.Detail]Network)
	config := make(map[mlmodel.Detail]mlmodel.Model)

	multi := make([]Network, 0)
	multiHash := make([]string, 0)
	multiType := make([]string, 0)
	for i, net := range gen {
		nw, cfg := net()
		if cfg.Live {
			nvw, _ := net()
			multi = append(multi, nvw)
			multiType = append(multiType, cfg.Detail.Type)
			multiHash = append(multiHash, cfg.Detail.Hash)
		}
		hash := cfg.Detail.Hash
		if hash == "" {
			hash = coinmath.String(5)
		}
		detail := mlmodel.Detail{
			Type:  networkType(nw),
			Hash:  hash,
			Index: i,
		}

		// check if we load the network
		loadErr := nw.Load(key, detail)
		log.Info().Err(loadErr).
			Str("key", key.ToString()).
			Str("detail", detail.ToString()).
			Msg("loaded network")

		networks[detail] = nw
		config[detail] = cfg
		trackers[detail] = NewTracker(12)
	}

	multiDetail := mlmodel.Detail{
		Type:  strings.Join(multiType, ":"),
		Hash:  strings.Join(multiHash, "|"),
		Index: len(gen),
	}
	networks[multiDetail] = NewMultiNetwork(multi...)
	config[multiDetail] = mlmodel.Model{}
	trackers[multiDetail] = NewTracker(12)

	return &BaseNetwork{
		set:    NewDataSet(in, out),
		config: config,
		net:    networks,
		track:  trackers,
	}
}

// Push receives a vector event and processes it with the provided network as a input-output tensor
func (b *BaseNetwork) Push(k model.Key, vv mlmodel.Vector) (map[mlmodel.Detail][][]float64, bool, error) {
	in, ready, _ := b.set.Push(k, vv)
	if ready {
		out := make(map[mlmodel.Detail][][]float64, len(b.net))
		minLoss := math.MaxFloat64
		for detail, net := range b.net {
			// append the detail now to have it for later ...
			trainDetails, trainErr := net.Train(b.set.In, b.set.Out)
			if trainErr != nil {
				log.Warn().
					Str("key", k.ToString()).
					Str("details", fmt.Sprintf("%+v", trainDetails)).
					Str("detail", fmt.Sprintf("%+v", detail)).
					Err(trainErr).
					Msg("training failed")
			} else {
				// assess the performance
				lastPrediction := b.track[detail].stats.Prediction
				if lastPrediction != nil && len(lastPrediction) > 0 {
					lossVec := net.Loss(b.set.Out, lastPrediction)
					loss := xmath.Vec(len(lossVec)).With(lossVec...).Norm()
					if _, ok := b.track[detail].stats.Loss.Push(loss); ok {
						//lossHistory := b.track[detail].stats.Loss.GetAsFloats(true)[0]
						b.track[detail].metrics.Total += 1
						isRelevant := math.Abs(lastPrediction[len(lastPrediction)-1][0]) > 0.5
						if isRelevant {
							if loss < 0.5 {
								b.track[detail].metrics.Match += 1
							} else if loss > 0.5 {
								b.track[detail].metrics.False += 1
							}
						}

						// TODD : naive logic for choosing the best network
						b.track[detail].metrics.Loss = loss
						if loss < minLoss {
							minLoss = loss
						}
						//fmt.Printf("b.track[%v].metrics = %+v\n", detail, b.track[detail].metrics)
					}
				}
				// do the next prediction
				thisOut, predictDetails, predictErr := net.Predict(in)
				b.track[detail].SetLast(thisOut)
				if predictErr != nil {
					log.Warn().
						Str("key", k.ToString()).
						Str("details", fmt.Sprintf("%+v", predictDetails)).
						Str("detail", fmt.Sprintf("%+v", detail)).
						Err(predictErr).
						Msg("prediction failed")
				} else {
					out[detail] = thisOut
					if minLoss == math.MaxFloat64 {
						// init the first to have something to work with in the first iterations
					}
					// save the network state
					saveErr := net.Save(k, detail)
					if saveErr != nil {
						log.Warn().Err(saveErr).
							Str("key", k.ToString()).
							Str("detail", detail.ToString()).
							Msg("could not save network")
					}
					// set a default accuracy if nothing is there ...
					if b.track[detail].metrics.Loss == 0.0 {
						b.track[detail].metrics.Loss = 0.01
					}
				}
			}
		}
		// pick the best detail
		return out, true, nil
	}
	return nil, false, nil

}

// Tracker defines a base network implementation
type Tracker struct {
	stats   Stats
	metrics Performance
}

// NewTracker creates a new single network
func NewTracker(size int) *Tracker {
	return &Tracker{
		stats:   NewStats(size),
		metrics: Performance{},
	}
}

func (bn *Tracker) SetLast(last [][]float64) {
	bn.stats.Prediction = last
}

type Performance struct {
	Total    int
	Match    int
	False    int
	Detail   mlmodel.Detail
	Accuracy float64
	Loss     float64
}

type Performances []Performance

func (u Performances) Len() int {
	return len(u)
}
func (u Performances) Swap(i, j int) {
	u[i], u[j] = u[j], u[i]
}
func (u Performances) Less(i, j int) bool {
	return u[i].Accuracy > u[j].Accuracy
}
