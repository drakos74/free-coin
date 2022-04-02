package ml

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	coinml "github.com/drakos74/free-coin/internal/math/ml"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/go-ex-machina/xmachina/ml"
	"github.com/drakos74/go-ex-machina/xmachina/net"
	"github.com/drakos74/go-ex-machina/xmachina/net/ff"
	"github.com/drakos74/go-ex-machina/xmath"
	"github.com/rs/zerolog/log"
)

type Network interface {
	Train(config Model, ds *dataset) (model.Type, float64, bool)
	Fit(config Model, ds *dataset) (float64, error)
	Predict(config Model, ds *dataset) model.Type
}

type RandomForestNetwork struct {
	debug bool
}

func (r RandomForestNetwork) Train(config Model, ds *dataset) (model.Type, float64, bool) {
	acc, err := r.Fit(config, ds)
	if err != nil {
		log.Error().Err(err).Msg("could not train online")
	} else if acc > config.PrecisionThreshold {
		t := r.Predict(config, ds)
		if t != model.NoType {
			return t, acc, true
		}
	}
	return model.NoType, 0, false
}

func (r RandomForestNetwork) Fit(config Model, ds *dataset) (float64, error) {
	hash := "tmp_fit"
	if r.debug {
		hash = time.Now().Format(time.RFC3339)
	}
	fn, err := toFeatureFile(trainDataSetPath, ds.getDescription(fmt.Sprintf("forest_%s", hash)), ds.vectors, false)
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

func (r RandomForestNetwork) Predict(config Model, ds *dataset) model.Type {
	fn, err := toFeatureFile(predictDataSetPath, ds.getDescription(fmt.Sprintf("forest_%s", "tmp_predict")), ds.vectors, true)
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
}

type datasets struct {
	sets    map[model.Key]*dataset
	network Network
}

func newDataSets(network Network) *datasets {
	return &datasets{
		sets:    make(map[model.Key]*dataset),
		network: network,
	}
}

func (ds *datasets) push(key model.Key, vv vector, cfg Model) (*dataset, bool) {
	if _, ok := ds.sets[key]; !ok {
		ds.sets[key] = newDataSet(key.Coin, key.Duration, cfg, make([]vector, 0))
	}
	// keep only the last vectors based on the buffer size
	newVectors := addVector(ds.sets[key].vectors, vv, cfg.BufferSize)

	ds.sets[key] = newDataSet(key.Coin, key.Duration, cfg, newVectors)

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

func newDataSet(coin model.Coin, duration time.Duration, cfg Model, vv []vector) *dataset {
	return &dataset{
		coin:     coin,
		duration: duration,
		vectors:  vv,
		config:   cfg,
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
	net *ff.Network
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

func (n *NNetwork) Train(config Model, ds *dataset) (model.Type, float64, bool) {

	accuracy := math.MaxFloat64
	i := 0
	//for {
	acc, err := n.Fit(config, ds)
	if err != nil {
		log.Error().Err(err).Msg("error during training")
		return model.NoType, 0.0, false
	}
	//if acc < 1 || i > 10 {
	accuracy = acc
	//break
	//}
	i++
	//}

	//if i < 10 && accuracy < 1 {
	t := n.Predict(config, ds)

	return t, accuracy, t != model.NoType
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
	fmt.Printf("l = %+v\n", l)
	return l, nil
}

func (n *NNetwork) Predict(config Model, ds *dataset) model.Type {

	last := ds.vectors[len(ds.vectors)-1]

	inp := xmath.Vec(len(last.newIn)).With(last.newIn...)

	outp := n.net.Predict(inp)

	fmt.Printf("outp = %+v\n", outp)

	if outp[0]-outp[2] > 0.0 {
		return model.Buy
	} else if outp[2]-outp[0] > 0.0 {
		return model.Sell
	}

	return model.NoType

}
