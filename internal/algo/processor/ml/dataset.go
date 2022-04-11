package ml

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/drakos74/free-coin/client"
	coinmath "github.com/drakos74/free-coin/internal/math"
	coinml "github.com/drakos74/free-coin/internal/math/ml"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/go-ex-machina/xmachina/ml"
	"github.com/drakos74/go-ex-machina/xmachina/net"
	"github.com/drakos74/go-ex-machina/xmachina/net/ff"
	"github.com/drakos74/go-ex-machina/xmath"
	"github.com/rs/zerolog/log"
)

type Network interface {
	Train(config Model, ds *dataset) (modelResult, map[string]modelResult)
	Fit(config Model, ds *dataset) (float64, error)
	Predict(config Model, ds *dataset) model.Type
	Eval(k string, report client.Report)
	Report() client.Report
}

type ConstructNetwork func() Network

type SingleNetwork struct {
	report client.Report
}

func (bn *SingleNetwork) Eval(k string, report client.Report) {
	bn.report = report
}

func (bn *SingleNetwork) Report() client.Report {
	return bn.report
}

type RandomForestNetwork struct {
	SingleNetwork
	debug  bool
	tmpKey string
}

func ConstructRandomForest(debug bool) func() Network {
	return func() Network {
		return NewRandomForestNetwork(debug, coinmath.String(10))
	}
}

func NewRandomForestNetwork(debug bool, key string) *RandomForestNetwork {
	return &RandomForestNetwork{debug: debug, tmpKey: key}
}

func (r *RandomForestNetwork) Train(config Model, ds *dataset) (modelResult, map[string]modelResult) {
	acc, err := r.Fit(config, ds)
	if err != nil {
		log.Error().Err(err).Msg("could not train online")
	} else if acc > config.PrecisionThreshold {
		t := r.Predict(config, ds)
		if t != model.NoType {
			return modelResult{
				key: r.tmpKey,
				t:   t,
				acc: acc,
				ok:  true,
			}, make(map[string]modelResult)
		}
	}
	return modelResult{}, make(map[string]modelResult)
}

func (r *RandomForestNetwork) Fit(config Model, ds *dataset) (float64, error) {
	hash := r.tmpKey
	fn, err := toFeatureFile(trainDataSetPath, ds.getDescription(fmt.Sprintf("forest_%s_%s", hash, "tmp_train")), ds.vectors, false)
	if err != nil {
		log.Error().Err(err).Msg("could not create dataset file")
		return 0.0, err
	}

	_, _, prec, err := coinml.RandomForestTrain(fn, config.ModelSize, config.Features, r.debug)
	if err != nil {
		log.Error().Err(err).Msg("could not train with isolation forest")
		return 0.0, err
	}
	return prec, nil
}

func (r *RandomForestNetwork) Predict(config Model, ds *dataset) model.Type {
	hash := r.tmpKey
	fn, err := toFeatureFile(predictDataSetPath, ds.getDescription(fmt.Sprintf("forest_%s_%s", hash, "tmp_predict")), ds.vectors, true)
	if err != nil {
		log.Error().Err(err).Msg("could not create dataset file")
		return model.NoType
	}

	predictions, err := coinml.RandomForestPredict(fn, config.ModelSize, config.Features, false)
	if err != nil {
		log.Error().Err(err).Msg("could not train with isolation forest")
		return model.NoType
	}
	_, a := predictions.Size()
	lastPrediction := predictions.RowString(a - 1)
	return model.TypeFromString(lastPrediction)
}

type dataset struct {
	coin     model.Coin
	duration time.Duration
	vectors  []vector
	config   Model
	network  Network
}

type datasets struct {
	sets    map[model.Key]*dataset
	network func() Network
}

func newDataSets(network func() Network) *datasets {
	return &datasets{
		sets:    make(map[model.Key]*dataset),
		network: network,
	}
}

func (ds *datasets) push(key model.Key, vv vector, cfg Model) (*dataset, bool) {
	if _, ok := ds.sets[key]; !ok {
		ds.sets[key] = newDataSet(key.Coin, key.Duration, cfg, make([]vector, 0), ds.network())
	}
	// keep only the last vectors based on the buffer size
	newVectors := addVector(ds.sets[key].vectors, vv, cfg.BufferSize)

	ds.sets[key] = newDataSet(key.Coin, key.Duration, cfg, newVectors, ds.sets[key].network)

	if len(ds.sets[key].vectors) >= cfg.BufferSize {
		return ds.sets[key], true
	}
	return &dataset{}, false
}

func addVector(ss []vector, s vector, size int) []vector {
	newVectors := append(ss, s)
	l := len(newVectors)
	if l > size {
		newVectors = newVectors[l-size:]
	}
	return newVectors
}

func newDataSet(coin model.Coin, duration time.Duration, cfg Model, vv []vector, network Network) *dataset {
	return &dataset{
		coin:     coin,
		duration: duration,
		vectors:  vv,
		config:   cfg,
		network:  network,
	}
}

func (s dataset) getDescription(postfix string) string {
	return fmt.Sprintf("%s_%s_%.2f_%s", s.coin, s.duration, s.config.PrecisionThreshold, postfix)
}

const benchmarkModelPath = "file-storage/ml/models"
const trainDataSetPath = "file-storage/ml/datasets"
const predictDataSetPath = "file-storage/ml/tmp"

func toFeatureFile(parentPath string, description string, vectors []vector, predict bool) (string, error) {
	fn, err := makePath(parentPath, fmt.Sprintf("%s.csv", description))
	if err != nil {
		return "", err
	}
	file, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	defer file.Close()

	if err != nil {
		return "", fmt.Errorf("could not open file: %w", err)
	}

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// take only the last n samples
	for _, vector := range vectors {
		lw := new(strings.Builder)
		for _, in := range vector.prevIn {
			lw.WriteString(fmt.Sprintf("%f,", in))
		}
		if vector.prevOut[0] == 1.0 {
			lw.WriteString(fmt.Sprintf("%s", model.Buy.String()))
		} else if vector.prevOut[2] == 1.0 {
			lw.WriteString(fmt.Sprintf("%s", model.Sell.String()))
		} else {
			lw.WriteString(fmt.Sprintf("%s", model.NoType.String()))
		}
		_, _ = writer.WriteString(lw.String() + "\n")
	}
	if predict {
		// for the last one add also the new value ...
		lastVector := vectors[len(vectors)-1]
		pw := new(strings.Builder)
		for _, in := range lastVector.newIn {
			pw.WriteString(fmt.Sprintf("%f,", in))
		}
		pw.WriteString(fmt.Sprintf("%s", model.NoType.String()))
		_, _ = writer.WriteString(pw.String() + "\n")
	}
	return fn, nil
}

func makePath(parentDir string, fileName string) (string, error) {
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		err := os.MkdirAll(parentDir, 0700) // Create your file
		if err != nil {
			return "", err
		}
	}
	fileName = fmt.Sprintf("%s/%s", parentDir, fileName)
	//file, _ := os.Create(fileName)
	//defer file.Close()
	return fileName, nil
}

// NNetwork defines an ml network.
type NNetwork struct {
	SingleNetwork
	net *ff.Network
}

func ConstructNeuralNetwork(network ff.Network) func() Network {
	return func() Network {
		return NewNN(&network)
	}
}

// NewNN creates a new neural network.
func NewNN(network *ff.Network) *NNetwork {
	// tanh with softmax
	if network == nil {
		rate := ml.Learn(1, 0.1)

		initW := xmath.Rand(-1, 1, math.Sqrt)
		initB := xmath.Rand(-1, 1, math.Sqrt)
		network = ff.New(6, 3).
			Add(48, net.NewBuilder().
				WithModule(ml.Base().
					WithRate(rate).
					WithActivation(ml.TanH)).
				WithWeights(initW, initB).
				Factory(net.NewActivationCell)).
			Add(9, net.NewBuilder().
				WithModule(ml.Base().
					WithRate(rate).
					WithActivation(ml.TanH)).
				WithWeights(initW, initB).
				Factory(net.NewActivationCell)).
			Add(3, net.NewBuilder().
				WithModule(ml.Base().
					WithRate(rate).
					WithActivation(ml.TanH)).
				WithWeights(initW, initB).
				Factory(net.NewActivationCell)).
			Add(3, net.NewBuilder().CellFactory(net.NewSoftCell))
		network.Loss(ml.Pow)
	}

	return &NNetwork{net: network}
}

func (n *NNetwork) Train(config Model, ds *dataset) (modelResult, map[string]modelResult) {

	accuracy := math.MaxFloat64
	i := 0
	//for {
	acc, err := n.Fit(config, ds)
	if err != nil {
		log.Error().Err(err).Msg("error during training")
		return modelResult{}, make(map[string]modelResult)
	}
	//if acc < 1 || i > 10 {
	accuracy = acc
	//break
	//}
	i++
	//}

	//if i < 10 && accuracy < 1 {
	t := n.Predict(config, ds)

	return modelResult{
		t:   t,
		acc: accuracy,
		ok:  t != model.NoType,
	}, make(map[string]modelResult)
	//}
	//
	//return model.NoType, 0.0, false

}

func (n *NNetwork) Fit(config Model, ds *dataset) (float64, error) {
	l := 0.0
	for i := 0; i < len(ds.vectors)-1; i++ {
		vv := ds.vectors[i]
		inp := xmath.Vec(len(vv.prevIn)).With(vv.prevIn...)
		loss, _ := n.net.Train(inp, xmath.Vec(len(vv.prevOut)).With(vv.prevOut...))
		l += loss.Norm()
	}
	return l, nil
}

func (n *NNetwork) Predict(config Model, ds *dataset) model.Type {

	last := ds.vectors[len(ds.vectors)-1]

	inp := xmath.Vec(len(last.newIn)).With(last.newIn...)

	outp := n.net.Predict(inp)

	if outp[0]-outp[2] > 0.0 {
		return model.Buy
	} else if outp[2]-outp[0] > 0.0 {
		return model.Sell
	}

	return model.NoType

}

type PolynomialRegression struct {
	SingleNetwork
	threshold float64
}

func ConstructPolynomialNetwork(threshold float64) func() Network {
	return func() Network {
		return NewPolynomialRegressionNetwork(threshold)
	}
}

func NewPolynomialRegressionNetwork(threshold float64) *PolynomialRegression {
	return &PolynomialRegression{threshold: threshold}
}

func (p *PolynomialRegression) Train(config Model, ds *dataset) (modelResult, map[string]modelResult) {
	if len(ds.vectors) > 0 {
		v := ds.vectors[len(ds.vectors)-1]
		in := v.newIn
		if in[2] > p.threshold {
			return modelResult{
				t:   model.Buy,
				acc: in[2],
				ok:  true,
			}, make(map[string]modelResult)
		} else if in[2] < -1*p.threshold {
			return modelResult{
				t:   model.Sell,
				acc: math.Abs(in[2]),
				ok:  true,
			}, make(map[string]modelResult)
		}
	}
	return modelResult{}, make(map[string]modelResult)
}

func (p *PolynomialRegression) Fit(config Model, ds *dataset) (float64, error) {
	panic("implement me")
}

func (p *PolynomialRegression) Predict(config Model, ds *dataset) model.Type {
	panic("implement me")
}

type MultiNetwork struct {
	ID        string
	construct map[string]ConstructNetwork
	networks  map[string]Network
}

func NewMultiNetwork(network ...ConstructNetwork) *MultiNetwork {
	nn := make(map[string]Network)
	cc := make(map[string]ConstructNetwork)
	for i, net := range network {
		k := fmt.Sprintf("%+v", i)
		cc[k] = net
		nn[k] = net()
	}
	return &MultiNetwork{
		ID:        coinmath.String(5),
		networks:  nn,
		construct: cc,
	}
}

type modelResult struct {
	key    string
	t      model.Type
	acc    float64
	profit float64
	ok     bool
	reset  bool
}

type modelResults []modelResult

func (rr modelResults) Len() int           { return len(rr) }
func (rr modelResults) Less(i, j int) bool { return rr[i].profit < rr[j].profit }
func (rr modelResults) Swap(i, j int)      { rr[i], rr[j] = rr[j], rr[i] }

func (m *MultiNetwork) Train(config Model, ds *dataset) (modelResult, map[string]modelResult) {

	tt := make(map[string]modelResult)

	results := make([]modelResult, 0)

	kk := make(map[string]client.Report, 0)

	for k, net := range m.networks {
		report := net.Report()
		res, _ := net.Train(config, ds)
		result := modelResult{
			key:    k,
			t:      res.t,
			acc:    res.acc,
			profit: report.Profit,
			ok:     res.ok,
		}
		if res.ok && report.Profit >= 0.0 {
			results = append(results, result)
		} else if res.ok && report.Profit < -0.1 {
			kk[k] = report
			result.reset = true
		}
		if res.t != model.NoType {
			tt[k] = result
		}
	}

	for k, report := range kk {
		log.Info().
			Str("key", k).
			Int("trades", report.Buy+report.Sell).
			Float64("profit", report.Profit).
			Msg("new network")
		m.networks[k] = m.construct[k]()
	}

	if len(results) == 0 {
		return modelResult{}, tt
	}

	sort.Sort(sort.Reverse(modelResults(results)))

	return results[0], tt
}

func (m *MultiNetwork) Eval(k string, report client.Report) {
	for key, n := range m.networks {
		if k == key {
			n.Eval(k, report)
		}
	}
}

func (m *MultiNetwork) Report() client.Report {
	return client.Report{}
}

func (m *MultiNetwork) Fit(config Model, ds *dataset) (float64, error) {
	panic("implement me")
}

func (m *MultiNetwork) Predict(config Model, ds *dataset) model.Type {
	panic("implement me")
}
