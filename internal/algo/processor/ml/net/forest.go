package net

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sjwhitworth/golearn/base"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	coinmath "github.com/drakos74/free-coin/internal/math"
	coinml "github.com/drakos74/free-coin/internal/math/ml"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

type RandomForestNetwork struct {
	SingleNetwork
	cfg    mlmodel.Model
	debug  bool
	tmpKey string
	tree   base.Classifier
}

func ConstructRandomForest(debug bool) func(cfg mlmodel.Model) Network {
	return func(cfg mlmodel.Model) Network {
		config := cfg.Evolve()
		return NewRandomForestNetwork(debug, coinmath.String(10), config)
	}
}

func NewRandomForestNetwork(debug bool, key string, cfg mlmodel.Model) *RandomForestNetwork {
	return &RandomForestNetwork{
		SingleNetwork: NewSingleNetwork(),
		cfg:           cfg,
		debug:         debug,
		tmpKey:        key,
	}
}

func (r *RandomForestNetwork) Model() mlmodel.Model {
	return r.cfg
}

func (r *RandomForestNetwork) Train(ds *Dataset) ModelResult {
	config := r.cfg
	acc, err := r.Fit(ds)

	r.statsCollector.Iterations++
	if err != nil {
		log.Error().Err(err).Msg("could not train online")
	} else if acc > config.PrecisionThreshold {
		t := r.Predict(ds)
		if t != model.NoType {
			r.statsCollector.History.Push(acc, float64(t))
			return ModelResult{
				Detail: mlmodel.Detail{
					Type: networkType(r),
					Hash: r.tmpKey,
				},
				Type:     t,
				Accuracy: acc,
				OK:       true,
			}
		}
	} else {
		r.tree = nil
	}
	r.statsCollector.History.Push(acc, 0)
	return ModelResult{}
}

func (r *RandomForestNetwork) Fit(ds *Dataset) (float64, error) {
	config := r.cfg
	hash := r.tmpKey
	vv := ds.Vectors
	if len(vv) > r.cfg.BufferSize {
		vv = vv[len(ds.Vectors)-r.cfg.BufferSize:]
	}
	fn, err := toFeatureFile(trainDataSetPath, ds.getDescription(fmt.Sprintf("forest_%s_%s", hash, "tmp_train")), vv, false)
	if err != nil {
		log.Error().Err(err).Msg("could not create Dataset file")
		return 0.0, err
	}

	tree, _, prec, err := coinml.RandomForestTrain(r.tree, fn, config.ModelSize, config.Features, r.debug)
	if err != nil {
		log.Error().Err(err).Msg("could not train with isolation forest")
		return 0.0, err
	}
	r.tree = tree
	return prec, nil
}

func (r *RandomForestNetwork) Predict(ds *Dataset) model.Type {
	hash := r.tmpKey
	vv := ds.Vectors
	if len(vv) > r.cfg.BufferSize {
		vv = vv[len(ds.Vectors)-r.cfg.BufferSize:]
	}
	fn, err := toFeatureFile(predictDataSetPath, ds.getDescription(fmt.Sprintf("forest_%s_%s", hash, "tmp_predict")), vv, true)
	if err != nil {
		log.Error().Err(err).Msg("could not create Dataset file")
		return model.NoType
	}

	if r.tree == nil {
		log.Error().Err(err).Msg("no tree trained")
		return model.NoType
	}

	predictions, err := coinml.RandomForestPredict(r.tree, fn, false)
	if err != nil {
		log.Error().Err(err).Msg("could not train with isolation forest")
		return model.NoType
	}
	_, a := predictions.Size()
	lastPrediction := predictions.RowString(a - 1)
	return model.TypeFromString(lastPrediction)
}

const benchmarkModelPath = "file-storage/ml/models"
const trainDataSetPath = "file-storage/ml/datasets"
const predictDataSetPath = "file-storage/ml/tmp"

func toFeatureFile(parentPath string, description string, vectors []mlmodel.Vector, predict bool) (string, error) {
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
		for _, in := range vector.PrevIn {
			lw.WriteString(fmt.Sprintf("%f,", in))
		}
		if vector.PrevOut[0] == 1.0 {
			lw.WriteString(fmt.Sprintf("%s", model.Buy.String()))
		} else if vector.PrevOut[2] == 1.0 {
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
		for _, in := range lastVector.NewIn {
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
