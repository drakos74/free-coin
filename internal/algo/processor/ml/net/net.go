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
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
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
	Train(x []float64, result int, train bool) error
	Predict(x []float64) int
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

type MultiNetwork struct {
	ID        string
	construct map[mlmodel.Detail]ConstructNetwork
	Networks  map[mlmodel.Detail]Network
	Evolution map[mlmodel.Detail]*buffer.MultiBuffer
	Trend     map[mlmodel.Detail]float64
	XY        map[mlmodel.Detail][][]float64
	cfg       mlmodel.Model
	Stats     map[mlmodel.Detail]Stat
	CC        map[mlmodel.Detail][]mlmodel.Performance
}

func newBuffer() *buffer.MultiBuffer {
	return buffer.NewMultiBuffer(3)
}

func NewMultiNetwork(cfg mlmodel.Model, network ...ConstructNetwork) *MultiNetwork {
	nn := make(map[mlmodel.Detail]Network)
	cc := make(map[mlmodel.Detail]ConstructNetwork)
	ev := make(map[mlmodel.Detail]*buffer.MultiBuffer)
	tt := make(map[mlmodel.Detail]float64)
	xy := make(map[mlmodel.Detail][][]float64)
	for i, net := range network {
		nnet := net(cfg)
		k := mlmodel.Detail{
			Type:  networkType(nnet),
			Index: i,
		}
		cc[k] = net
		nn[k] = nnet
		ev[k] = newBuffer()
	}
	return &MultiNetwork{
		ID:        coinmath.String(5),
		Networks:  nn,
		construct: cc,
		Evolution: ev,
		Trend:     tt,
		XY:        xy,
		cfg:       cfg,
		Stats:     make(map[mlmodel.Detail]Stat),
		CC:        make(map[mlmodel.Detail][]mlmodel.Performance),
	}
}

func MultiNetworkConstructor(network ...ConstructNetwork) ConstructMultiNetwork {
	return func(cfg mlmodel.Model) *MultiNetwork {
		return NewMultiNetwork(cfg, network...)
	}
}

type ModelResult struct {
	Detail             mlmodel.Detail
	Type               model.Type
	Gap                float64
	Accuracy           float64
	Profit             float64
	Trend              float64
	XY                 [][]float64
	Features           []float64
	FeaturesImportance []float64
	OK                 bool
	Reset              bool
}

func (r ModelResult) Decision() *model.Decision {
	return &model.Decision{
		Confidence: r.Accuracy,
		Features:   r.Features,
		Importance: r.FeaturesImportance,
		Config:     []float64{r.Gap, r.Profit, r.Trend},
	}
}

type modelResults []ModelResult

func (rr modelResults) Len() int           { return len(rr) }
func (rr modelResults) Less(i, j int) bool { return rr[i].Trend < rr[j].Trend }
func (rr modelResults) Swap(i, j int)      { rr[i], rr[j] = rr[j], rr[i] }

func (m *MultiNetwork) Train(ds *Dataset) (ModelResult, map[mlmodel.Detail]ModelResult) {

	results := make([]ModelResult, 0)

	tt := make(map[mlmodel.Detail]ModelResult)
	kk := make(map[mlmodel.Detail]client.Report, 0)
	cfgs := make(map[mlmodel.Detail]mlmodel.Model, 0)

	for k, net := range m.Networks {
		// make sure we train only models fit for the dataset
		if len(ds.Vectors) < net.Model().BufferSize {
			continue
		}

		report := net.Report()
		res := net.Train(ds)

		var trend float64
		xy := [][]float64{make([]float64, 0), make([]float64, 0)}
		detail := mlmodel.NetworkDetail(k.Type)

		if m.Evolution[k].Len() >= 3 {
			vv := m.Evolution[k].Get()
			if len(vv) >= 2 {
				xx := make([]float64, 0)
				yy := make([]float64, 0)
				for i, v := range vv {
					xx = append(xx, float64(i))
					yy = append(yy, math.Round(v[1]))
				}
				a, err := coinmath.Fit(xx, yy, 2)
				if err == nil {
					trend = a[2]
					m.Trend[k] = trend
					xy = [][]float64{xx, yy}
				}
			}
			m.XY[k] = xy
		}

		result := ModelResult{
			Detail:             k,
			Type:               res.Type,
			Accuracy:           res.Accuracy,
			Features:           res.Features,
			FeaturesImportance: res.FeaturesImportance,
			Profit:             report.Profit,
			Trend:              trend,
			XY:                 xy,
			OK:                 res.OK,
		}
		// TODO : make this configurable
		if res.OK && result.Profit > 1.0 && result.Trend > 0.1 {
			results = append(results, result)
			// add the config to the winners
			m.assessPerformance(detail, result, net.Model().ToSlice())
		} else if res.OK && result.Trend < -0.1 {
			kk[k] = report
			result.Reset = true
		}
		m.trackStats(detail, result.Profit, result.Reset)
		if res.Type != model.NoType {
			tt[k] = result
			cfgs[k] = net.Model()
		}
	}

	for k, report := range kk {
		if len(m.CC[mlmodel.NetworkDetail(k.Type)]) > 0 {
			m.cfg = mlmodel.EvolveModel(m.CC[mlmodel.NetworkDetail(k.Type)])
		}

		m.Networks[k] = m.construct[k](m.cfg)
		m.Evolution[k] = newBuffer()
		m.Trend[k] = 0.0
		m.XY[k] = [][]float64{make([]float64, 0), make([]float64, 0)}

		log.Info().
			Str("Index", fmt.Sprintf("%+v", k)).
			Str("coin", string(ds.Coin)).
			Str("duration", fmt.Sprintf("%+v", ds.Duration)).
			Int("trades", report.Buy+report.Sell).
			Float64("profit", report.Profit).
			Float64("trend", tt[k].Trend).
			Str("trend-xy", fmt.Sprintf("%+v", tt[k].XY)).
			Str("old_config", fmt.Sprintf("%+v", cfgs[k])).
			Str("new_config", fmt.Sprintf("%+v", m.Networks[k].Model())).
			Str("CC", fmt.Sprintf("%+v", m.CC)).
			Msg("replace network")
	}

	if len(results) == 0 {
		return ModelResult{}, tt
	}

	sort.Sort(sort.Reverse(modelResults(results)))

	return results[0], tt
}

func (m *MultiNetwork) assessPerformance(detail mlmodel.Detail, result ModelResult, config []float64) {
	if _, ok := m.CC[detail]; !ok {
		m.CC[detail] = make([]mlmodel.Performance, 0)
	}
	cc := m.CC[detail]
	cc = append(cc, mlmodel.Performance{
		Config: config,
		Score:  result.Profit,
	})
	sort.Sort(mlmodel.ByScore(cc))
	if len(cc) > 5 {
		cc = cc[1:]
	}
	m.CC[detail] = cc
}

func (m *MultiNetwork) trackStats(detail mlmodel.Detail, profit float64, reset bool) {
	if _, ok := m.Stats[detail]; !ok {
		m.Stats[detail] = Stat{
			Least: math.MaxFloat64,
		}
	}
	st := m.Stats[detail]
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

	m.Stats[detail] = st
}

func (m *MultiNetwork) Eval(k mlmodel.Detail, report client.Report) {
	for key, n := range m.Networks {
		if k == key {
			m.Evolution[key].Push(float64(time.Now().Unix()), report.Profit)
			n.Eval(report)
		}
	}
}

func (m *MultiNetwork) Model() mlmodel.Model {
	return m.cfg
}
