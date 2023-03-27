package net

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"time"

	"github.com/drakos74/free-coin/client"
	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/buffer"
	coinmath "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/math/ml"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	// benchmarkSamples defines how many samples of history to keep for model selection scores
	benchmarkSamples = 5
	// evolutionThreshold defines how many winner models to store and use for model evolution
	evolutionThreshold = 3
)

func networkType(net Network) string {
	return reflect.TypeOf(net).Elem().String()
}

// Stats defines generic network Stats.
type Stats struct {
	Iterations int
	Accuracy   []float64
	Decisions  []int
}

type StatsCollector struct {
	Iterations int
	History    *buffer.MultiBuffer
}

// NewStatsCollector creates a new Stats struct.
func NewStatsCollector(s int) *StatsCollector {
	return &StatsCollector{
		History: buffer.NewMultiBuffer(s),
	}
}

//Model defines a simplistic machine learning model
type Model interface {
	Train(x []float64, y float64, train bool) (ml.Metadata, error)
	Predict(x []float64, leadingThreshold int) (int, float64, ml.Metadata, error)
}

// Network defines the main interface for a network training.
// TODO : split network and multi-network interface
type Network interface {
	Train(ds *Dataset) ModelResult
	Eval(report client.Report)
	Report() client.Report
	Stats() Stats
	Model() mlmodel.Model
}

// ConstructNetwork defines a network constructor func.
type ConstructNetwork func(cfg mlmodel.Model) Network

// ConstructMultiNetwork defines a multi-network constructor func.
type ConstructMultiNetwork func(cfg mlmodel.Model) *MultiNetwork

// SingleNetwork defines a base network implementation
type SingleNetwork struct {
	report         client.Report
	config         mlmodel.Model
	statsCollector *StatsCollector
}

// NewSingleNetwork creates a new single network
func NewSingleNetwork(cfg mlmodel.Model) SingleNetwork {
	return SingleNetwork{
		statsCollector: NewStatsCollector(3),
		config:         cfg,
	}
}

func (bn *SingleNetwork) Eval(report client.Report) {
	bn.report = report
}

func (bn *SingleNetwork) Report() client.Report {
	return bn.report
}

func (bn *SingleNetwork) Model() mlmodel.Model {
	return bn.config
}

func (bn *SingleNetwork) Stats() Stats {

	history := bn.statsCollector.History.Get()

	acc := make([]float64, len(history))
	dec := make([]int, len(history))

	for i, h := range history {
		acc[i] = h[0]
		dec[i] = int(h[1])
	}

	return Stats{
		Iterations: bn.statsCollector.Iterations,
		Accuracy:   acc,
		Decisions:  dec,
	}
}

type Stat struct {
	Use   int
	Reset int
	Least float64
	Max   float64
}

type Net struct {
	Network   Network
	Benchmark *buffer.MultiBuffer
	Slope     float64
	Trend     float64
	XY        [][]float64
}

func newNet(cfg mlmodel.Model, network ConstructNetwork) *Net {
	return &Net{
		Network:   network(cfg),
		Benchmark: newBuffer(benchmarkSamples),
		Slope:     0,
		Trend:     0,
		XY:        make([][]float64, 0),
	}
}

func (n *Net) setSlope(slope float64) {
	n.Slope = slope
}

func (n *Net) setTrend(trend float64) {
	n.Trend = trend
}

func (n *Net) setXY(xy [][]float64) {
	n.XY = xy
}

type Config struct {
	config      mlmodel.Model
	construct   ConstructNetwork
	Stat        Stat
	Performance []mlmodel.Performance
}

func newConfig(cfg mlmodel.Model, network ConstructNetwork) *Config {
	return &Config{
		config:    cfg,
		construct: network,
		Stat: Stat{
			Least: math.MaxFloat64,
		},
		Performance: make([]mlmodel.Performance, 0),
	}
}

type MultiNetwork struct {
	ID       string
	Networks map[mlmodel.Detail]*Net
	Config   map[string]*Config
}

func newBuffer(size int) *buffer.MultiBuffer {
	return buffer.NewMultiBuffer(size)
}

func NewMultiNetwork(cfg mlmodel.Model, network ...ConstructNetwork) *MultiNetwork {

	config := make(map[string]*Config)
	nets := make(map[mlmodel.Detail]*Net)

	for i, net := range network {
		nnet := net(cfg)
		detail := mlmodel.Detail{
			Type:  networkType(nnet),
			Hash:  coinmath.String(3),
			Index: i,
		}
		nets[detail] = newNet(cfg, net)
		// assign only the network type to the constructor, as the hash will change
		config[detail.Type] = newConfig(cfg, net)
	}
	return &MultiNetwork{
		ID:       coinmath.String(5),
		Networks: nets,
		Config:   config,
	}
}

func MultiNetworkConstructor(network ...ConstructNetwork) ConstructMultiNetwork {
	return func(cfg mlmodel.Model) *MultiNetwork {
		return NewMultiNetwork(cfg, network...)
	}
}

type ModelResult struct {
	Benchmark
	Detail             mlmodel.Detail
	Type               model.Type
	Gap                float64
	Accuracy           float64
	Features           []float64
	FeaturesImportance []float64
	Position           mlmodel.Position
	OK                 bool
	Reset              bool
}

type Benchmark struct {
	Profit  float64
	Trend   float64
	Slope   float64
	Actions int
	XY      [][]float64
}

func (r ModelResult) Decision() *model.Decision {
	return &model.Decision{
		Confidence: r.Accuracy,
		Features:   r.Features,
		Importance: r.FeaturesImportance,
		Config:     []float64{r.Gap, r.Profit, r.Trend},
		Position:   r.Position,
	}
}

type modelResults []ModelResult

func (rr modelResults) Len() int           { return len(rr) }
func (rr modelResults) Less(i, j int) bool { return rr[i].Slope < rr[j].Slope }
func (rr modelResults) Swap(i, j int)      { rr[i], rr[j] = rr[j], rr[i] }

func (m *MultiNetwork) Train(ds *Dataset) (ModelResult, map[mlmodel.Detail]ModelResult) {

	results := make([]ModelResult, 0)

	networkResults := make(map[mlmodel.Detail]ModelResult)
	networkReports := make(map[mlmodel.Detail]client.Report, 0)
	networkConfigs := make(map[mlmodel.Detail]mlmodel.Model, 0)

	for detail, net := range m.Networks {

		// make sure we train only models fit for the dataset
		if len(ds.Vectors) < net.Network.Model().BufferSize {
			log.Warn().
				Str("coin", string(ds.Coin)).
				Str("net", fmt.Sprintf("%+v", detail)).
				Int("vectors", len(ds.Vectors)).
				Int("buffer-size", net.Network.Model().BufferSize).
				Msg("not enough vectors to train")
			continue
		}

		report := net.Network.Report()
		res := net.Network.Train(ds)

		var trend float64
		var slope float64
		xy := [][]float64{make([]float64, 0), make([]float64, 0)}

		// create the set
		vv := net.Benchmark.Get()
		xx := make([]float64, 0)
		yy := make([]float64, 0)
		for i, v := range vv {
			if len(v) > 1 {
				xx = append(xx, float64(i))
				yy = append(yy, math.Round(v[1]))
			}

		}
		// fit on 1st degree
		if len(xx) > 1 && len(yy) > 1 {
			if a, err := coinmath.Fit(xx, yy, 1); err == nil {
				trend = a[1]
				m.Networks[detail].setTrend(trend)
				xy = [][]float64{xx, yy}
			} else {
				log.Error().
					Err(err).
					Str("coin", string(ds.Coin)).
					Str("duration", fmt.Sprintf("%+v", ds.Duration)).
					Floats64("xx", xx).
					Floats64("yy", yy).
					Msg("could not fit trend to 1st degree")
			}
		}
		// fit on 2nd degree
		if len(xx) > 2 && len(yy) > 2 {
			if b, err := coinmath.Fit(xx, yy, 2); err == nil {
				slope = b[2]
				m.Networks[detail].setSlope(slope)
			} else {
				log.Error().
					Err(err).
					Str("coin", string(ds.Coin)).
					Str("duration", fmt.Sprintf("%+v", ds.Duration)).
					Floats64("xx", xx).
					Floats64("yy", yy).
					Msg("could not fit trend to 2nd degree")
			}
		}
		m.Networks[detail].setXY(xy)

		result := ModelResult{
			Detail:             detail,
			Type:               res.Type,
			Accuracy:           res.Accuracy,
			Features:           res.Features,
			FeaturesImportance: res.FeaturesImportance,
			Benchmark: Benchmark{
				Profit:  report.Profit,
				Trend:   trend,
				Slope:   slope,
				XY:      xy,
				Actions: report.Sell + report.Buy,
			},
			OK: res.OK,
		}
		// TODO : make this configurable
		if res.OK && result.Profit > 1.0 && result.Trend > 0.1 {
			if report.Start.Unix() == 0 {
				report.Start = time.Now()
			}
			log.Info().
				Str("type", string(result.Type)).
				Str("detail", fmt.Sprintf("%+v", result.Detail)).
				Str("coin", string(ds.Coin)).
				Str("duration", fmt.Sprintf("%+v", ds.Duration)).
				Str("config", fmt.Sprintf("%+v", net.Network.Model())).
				Float64("trend", result.Trend).
				Float64("slope", result.Slope).
				Float64("profit", result.Profit).
				Int("mock-trades", result.Actions).
				Float64("accuracy", result.Accuracy).
				Int("benchmark-size", net.Benchmark.Len()).
				Str("benchmarks", fmt.Sprintf("%+v", net.Benchmark.Get())).
				Str("age", fmt.Sprintf("%+v", time.Now().Sub(report.Start).Minutes())).
				Msg("accept network")
			results = append(results, result)
			// add the config to the winners
			m.assessPerformance(detail.Type, result, net.Network.Model().ToSlice())
		} else if res.OK && result.Slope < -0.1 {
			report.Stop = time.Now()
			networkReports[detail] = report
			result.Reset = true
		}
		m.trackStats(detail.Type, result.Profit, result.Reset)
		if res.Type != model.NoType {
			networkResults[detail] = result
			networkConfigs[detail] = net.Network.Model()
		}
	}

	// replace the networks where applicable
	for k, report := range networkReports {
		if len(m.Config[k.Type].Performance) > 0 {
			// TODO : make this internal method ...
			// NOTE :no randomisation needed here, the constructor should randomise already
			m.Config[k.Type].config = mlmodel.EvolveModel(m.Config[k.Type].Performance)
		}

		newDetail := mlmodel.Detail{
			Type:  k.Type,
			Hash:  coinmath.String(5),
			Index: k.Index,
		}
		// note : first insert the new network
		m.Networks[newDetail] = newNet(m.Config[k.Type].config, m.Config[k.Type].construct)
		// clean up the old one
		delete(m.Networks, k)

		log.Info().
			Str("detail", fmt.Sprintf("%+v", k)).
			Str("coin", string(ds.Coin)).
			Str("duration", fmt.Sprintf("%+v", ds.Duration)).
			Int("trades", report.Buy+report.Sell).
			Float64("profit", report.Profit).
			Float64("trend", networkResults[k].Trend).
			Float64("slope", networkResults[k].Slope).
			Str("old_config", fmt.Sprintf("%+v", networkConfigs[k])).
			Str("new_config", fmt.Sprintf("%+v", m.Networks[newDetail].Network.Model())).
			Str("age", fmt.Sprintf("%+v", report.Stop.Sub(report.Start).Minutes())).
			Msg("replace network")

	}

	if len(results) == 0 {
		return ModelResult{}, networkResults
	}

	// pick the result with the highest trend
	sort.Sort(sort.Reverse(modelResults(results)))
	return results[0], networkResults
}

// assessPerformance keeps track of the winning models in order to support with evolution
func (m *MultiNetwork) assessPerformance(detail string, result ModelResult, config []float64) {
	cc := m.Config[detail].Performance
	cc = append(cc, mlmodel.Performance{
		Config: config,
		Score:  result.Profit,
	})
	sort.Sort(mlmodel.ByScore(cc))
	if len(cc) > evolutionThreshold {
		cc = cc[1:]
	}
	m.Config[detail].Performance = cc
}

func (m *MultiNetwork) trackStats(detail string, profit float64, reset bool) {
	st := m.Config[detail].Stat
	if profit < st.Least {
		st.Least = profit
	}
	if profit > st.Max {
		st.Max = profit
	}
	if reset {
		st.Reset = st.Reset + 1
	} else {
		st.Use = st.Use + 1
	}

	m.Config[detail].Stat = st
}

// Eval is called from outside to bring in the latest stats and updates on the performance of the networks
// TODO : clean up usage and clarify calling party reason ... if this is missing, networks cant evolve ...
func (m *MultiNetwork) Eval(detail mlmodel.Detail, report client.Report) {
	for d, net := range m.Networks {
		if detail == d {
			net.Benchmark.Push(float64(time.Now().Unix()), report.Profit)
			net.Network.Eval(report)
		}
	}
}
