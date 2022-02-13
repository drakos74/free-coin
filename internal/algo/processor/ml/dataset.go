package ml

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/math/ml"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

type dataset struct {
	coin     model.Coin
	duration time.Duration
	vectors  []vector
	config   Segments
}

type datasets struct {
	sets map[model.Key]dataset
}

func newDataSets() *datasets {
	return &datasets{
		sets: make(map[model.Key]dataset),
	}
}

func (ds *datasets) push(key model.Key, vv vector, segment Segments) (dataset, bool) {
	if _, ok := ds.sets[key]; !ok {
		ds.sets[key] = newDataSet(key.Coin, key.Duration, segment, make([]vector, 0))
	}
	// keep only the last vectors based on the buffer size
	newVectors := addVector(ds.sets[key].vectors, vv, segment.Model.BufferSize)

	ds.sets[key] = newDataSet(key.Coin, key.Duration, segment, newVectors)

	if len(ds.sets[key].vectors) >= segment.Model.BufferSize {
		return ds.sets[key], true
	}
	return dataset{}, false
}

func addVector(ss []vector, s vector, size int) []vector {
	newVectors := append(ss, s)
	l := len(newVectors)
	if l > size {
		newVectors = newVectors[l-size:]
	}
	return newVectors
}

func newDataSet(coin model.Coin, duration time.Duration, cfg Segments, vv []vector) dataset {
	return dataset{
		coin:     coin,
		duration: duration,
		vectors:  vv,
		config:   cfg,
	}
}

const benchmarkModelPath = "file-storage/ml/models"
const trainDataSetPath = "file-storage/ml/datasets"
const predictDataSetPath = "file-storage/ml/tmp"

func (s dataset) train(cfg Model) (model.Type, bool) {
	prec, err := s.fit(cfg, false)
	if err != nil {
		log.Error().Err(err).Msg("could not train online")
	} else if prec > cfg.PrecisionThreshold {
		t := s.predict(cfg)
		if t != model.NoType {
			return t, true
		}
	}
	return model.NoType, false
}

func (s dataset) predict(cfg Model) model.Type {
	fn, err := s.toFeatureFile(predictDataSetPath, fmt.Sprintf("forest_%s", "tmp_predict"), true)
	if err != nil {
		log.Error().Err(err).Msg("could not create dataset file")
		return model.NoType
	}

	predictions, err := ml.RandomForestPredict(fn, cfg.ModelSize, cfg.Features, false)
	if err != nil {
		log.Error().Err(err).Msg("could not train with isolation forest")
		return model.NoType
	}
	_, a := predictions.Size()
	lastPrediction := predictions.RowString(a - 1)
	return model.TypeFromString(lastPrediction)
}

func (s dataset) fit(cfg Model, debug bool) (float64, error) {
	hash := "tmp_fit"
	if debug {
		hash = time.Now().Format(time.RFC3339)
	}
	fn, err := s.toFeatureFile(trainDataSetPath, fmt.Sprintf("forest_%s", hash), false)
	if err != nil {
		log.Error().Err(err).Msg("could not create dataset file")
		return 0.0, err
	}

	_, _, prec, err := ml.RandomForestTrain(fn, cfg.ModelSize, cfg.Features, debug)
	if err != nil {
		log.Error().Err(err).Msg("could not train with isolation forest")
		return 0.0, err
	}
	return prec, nil
}

func (s dataset) getDescription(postfix string) string {
	return fmt.Sprintf("%s_%s_%.2f_%s", s.coin, s.duration, s.config.Model.PrecisionThreshold, postfix)
}

func (s dataset) toFeatureFile(parentPath string, postfix string, predict bool) (string, error) {

	fn, err := makePath(parentPath, fmt.Sprintf("%s.csv", s.getDescription(postfix)))
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
	for _, vector := range s.vectors {
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
		lastVector := s.vectors[len(s.vectors)-1]
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
