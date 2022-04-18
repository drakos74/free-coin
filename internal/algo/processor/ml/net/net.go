package net

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/drakos74/free-coin/client"
	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/buffer"
	coinmath "github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

// Stats defines generic network stats.
type Stats struct {
	Iterations int
	Accuracy   []float64
	Decisions  []int
}

type StatsCollector struct {
	Iterations int
	History    *buffer.MultiBuffer
}

// NewStatsCollector creates a new stats struct.
func NewStatsCollector(s int) *StatsCollector {
	return &StatsCollector{
		History: buffer.NewMultiBuffer(s),
	}
}

// Network defines the main interface for a network training.
// TODO : split network and multi-network interface
type Network interface {
	Train(ds *Dataset) (ModelResult, map[string]ModelResult)
	Fit(ds *Dataset) (float64, error)
	Predict(ds *Dataset) model.Type
	Eval(k string, report client.Report)
	Report() client.Report
	Stats() Stats
	Model() mlmodel.Model
}

// ConstructNetwork defines a network constructor func.
type ConstructNetwork func(cfg mlmodel.Model) Network

// SingleNetwork defines a base network implementation
type SingleNetwork struct {
	report         client.Report
	statsCollector *StatsCollector
}

func (bn *SingleNetwork) Eval(k string, report client.Report) {
	bn.report = report
}

func (bn *SingleNetwork) Report() client.Report {
	return bn.report
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

type MultiNetwork struct {
	ID        string
	construct map[string]ConstructNetwork
	Networks  map[string]Network
	Evolution map[string]*buffer.MultiBuffer
	Trend     map[string]float64
	cfg       mlmodel.Model
}

func newBuffer() *buffer.MultiBuffer {
	return buffer.NewMultiBuffer(3)
}

func NewMultiNetwork(cfg mlmodel.Model, network ...ConstructNetwork) *MultiNetwork {
	nn := make(map[string]Network)
	cc := make(map[string]ConstructNetwork)
	ev := make(map[string]*buffer.MultiBuffer)
	tt := make(map[string]float64)
	for i, net := range network {
		k := fmt.Sprintf("%+v", i)
		cc[k] = net
		nn[k] = net(cfg)
		ev[k] = newBuffer()

	}
	return &MultiNetwork{
		ID:        coinmath.String(5),
		Networks:  nn,
		construct: cc,
		Evolution: ev,
		Trend:     tt,
		cfg:       cfg,
	}
}

func MultiNetworkConstructor(network ...ConstructNetwork) func(cfg mlmodel.Model) Network {
	return func(cfg mlmodel.Model) Network {
		return NewMultiNetwork(cfg, network...)
	}
}

type ModelResult struct {
	Key      string
	Type     model.Type
	Accuracy float64
	Profit   float64
	Trend    float64
	OK       bool
	Reset    bool
}

type modelResults []ModelResult

func (rr modelResults) Len() int           { return len(rr) }
func (rr modelResults) Less(i, j int) bool { return rr[i].Trend < rr[j].Trend }
func (rr modelResults) Swap(i, j int)      { rr[i], rr[j] = rr[j], rr[i] }

func (m *MultiNetwork) Train(ds *Dataset) (ModelResult, map[string]ModelResult) {

	tt := make(map[string]ModelResult)

	results := make([]ModelResult, 0)

	kk := make(map[string]client.Report, 0)
	cfgs := make(map[string]mlmodel.Model, 0)

	cc := make([][]float64, 0)

	for k, net := range m.Networks {
		// make sure we train only models fit for the dataset
		if len(ds.Vectors) < net.Model().BufferSize {
			continue
		}

		report := net.Report()
		res, _ := net.Train(ds)

		var trend float64

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
				}
			}
		}

		result := ModelResult{
			Key:      k,
			Type:     res.Type,
			Accuracy: res.Accuracy,
			Profit:   report.Profit,
			Trend:    trend,
			OK:       res.OK,
		}
		if res.OK && result.Profit > 0 && result.Trend > 0 {
			results = append(results, result)
			cc = append(cc, net.Model().ToSlice())
		} else if res.OK && result.Trend < 0 {
			kk[k] = report
			result.Reset = true
		}
		if res.Type != model.NoType {
			tt[k] = result
			cfgs[k] = net.Model()
		}
	}

	for k, report := range kk {
		if len(cc) > 0 {
			m.cfg = mlmodel.EvolveModel(cc)
		}

		m.Networks[k] = m.construct[k](m.cfg)
		m.Evolution[k] = newBuffer()
		m.Trend[k] = 0.0

		log.Info().
			Str("Key", k).
			Str("coin", string(ds.Coin)).
			Str("duration", fmt.Sprintf("%+v", ds.Duration)).
			Int("trades", report.Buy+report.Sell).
			Float64("profit", report.Profit).
			Float64("trend", tt[k].Trend).
			Str("old_config", fmt.Sprintf("%+v", cfgs[k])).
			Str("new_config", fmt.Sprintf("%+v", m.Networks[k].Model())).
			Str("cc", fmt.Sprintf("%+v", cc)).
			Msg("replace network")
	}

	if len(results) == 0 {
		return ModelResult{}, tt
	}

	sort.Sort(sort.Reverse(modelResults(results)))

	return results[0], tt
}

func (m *MultiNetwork) Eval(k string, report client.Report) {
	for key, n := range m.Networks {
		if k == key {
			m.Evolution[key].Push(float64(time.Now().Unix()), report.Profit)
			n.Eval(k, report)
		}
	}
}

func (m *MultiNetwork) Report() client.Report {
	return client.Report{}
}

func (m *MultiNetwork) Fit(ds *Dataset) (float64, error) {
	panic("implement me")
}

func (m *MultiNetwork) Predict(ds *Dataset) model.Type {
	panic("implement me")
}

func (m *MultiNetwork) Model() mlmodel.Model {
	return m.cfg
}

func (m *MultiNetwork) Stats() Stats {
	return Stats{}
}
