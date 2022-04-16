package net

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
}

func ConstructRandomForest(debug bool) func(cfg mlmodel.Model) Network {
	return func(cfg mlmodel.Model) Network {
		config := cfg.Evolve()
		return NewRandomForestNetwork(debug, coinmath.String(10), config)
	}
}

func NewRandomForestNetwork(debug bool, key string, cfg mlmodel.Model) *RandomForestNetwork {
	return &RandomForestNetwork{debug: debug, tmpKey: key, cfg: cfg}
}

func (r *RandomForestNetwork) Model() mlmodel.Model {
	return r.cfg
}

func (r *RandomForestNetwork) Train(ds *Dataset) (ModelResult, map[string]ModelResult) {
	config := r.cfg
	acc, err := r.Fit(ds)
	if err != nil {
		log.Error().Err(err).Msg("could not train online")
	} else if acc > config.PrecisionThreshold {
		t := r.Predict(ds)
		if t != model.NoType {
			return ModelResult{
				Key:      r.tmpKey,
				Type:     t,
				Accuracy: acc,
				OK:       true,
			}, make(map[string]ModelResult)
		}
	}
	return ModelResult{}, make(map[string]ModelResult)
}

func (r *RandomForestNetwork) Fit(ds *Dataset) (float64, error) {
	config := r.cfg
	hash := r.tmpKey
	fn, err := toFeatureFile(trainDataSetPath, ds.getDescription(fmt.Sprintf("forest_%s_%s", hash, "tmp_train")), ds.Vectors, false)
	if err != nil {
		log.Error().Err(err).Msg("could not create Dataset file")
		return 0.0, err
	}

	_, _, prec, err := coinml.RandomForestTrain(fn, config.ModelSize, config.Features, r.debug)
	if err != nil {
		log.Error().Err(err).Msg("could not train with isolation forest")
		return 0.0, err
	}
	return prec, nil
}

func (r *RandomForestNetwork) Predict(ds *Dataset) model.Type {
	config := r.cfg
	hash := r.tmpKey
	fn, err := toFeatureFile(predictDataSetPath, ds.getDescription(fmt.Sprintf("forest_%s_%s", hash, "tmp_predict")), ds.Vectors, true)
	if err != nil {
		log.Error().Err(err).Msg("could not create Dataset file")
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

const benchmarkModelPath = "file-storage/ml/models"
const trainDataSetPath = "file-storage/ml/Datasets"
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
