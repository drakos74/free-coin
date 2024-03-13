package net

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/math/ml"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
	"github.com/sjwhitworth/golearn/base"
)

const FOREST_KEY string = "net.RandomForest"

type RandomForest struct {
	tree     base.Classifier
	template *base.DenseInstances
	metadata ml.Metadata
	cfg      mlmodel.Model
	buffer   *buffer.MultiBuffer
	debug    bool
	tmpKey   string
}

func NewRandomForest(cfg mlmodel.Model) *RandomForest {
	var tree base.Classifier
	return &RandomForest{
		tree:     tree,
		metadata: ml.NewMetadata(),
		cfg:      cfg,
		buffer:   buffer.NewMultiBuffer(cfg.BufferSize),
	}
}

func (r *RandomForest) Train(x [][]float64, y [][]float64) (ml.Metadata, error) {
	v := lastAt(0, y)
	w := append(last(x)[:r.cfg.Features[0]], quantify(v, r.cfg.Spread))
	if _, ok := r.buffer.Push(w...); ok {
		hash := r.tmpKey
		fn, err := toFeatureFile(trainDataSetPath, fmt.Sprintf("forest_%s_%s", hash, "tmp_train"), r.buffer.Get(), false)
		if err != nil {
			log.Error().Err(err).Msg("could not create Dataset file")
			return r.metadata, err
		}

		tree, template, _, err := ml.RandomForestTrain(r.tree, fn, r.cfg.Size[0], r.cfg.Features[0], r.debug)
		if err != nil {
			log.Error().Err(err).Msg("could not train with random forest")
			return r.metadata, err
		}
		r.template = template
		r.tree = tree
	} else {
		return r.metadata, fmt.Errorf("not enough samples to train tree : %d of %d", r.buffer.Len(), r.cfg.BufferSize)
	}
	return r.metadata, nil
}

func (r *RandomForest) Predict(x [][]float64) ([][]float64, ml.Metadata, error) {
	hash := r.tmpKey
	// add a new layer
	xx := append(make([][]float64, 0), append(last(x)[:r.cfg.Features[0]], 0))
	fmt.Printf("xx = %+v\n", xx)
	fn, err := toFeatureFile(predictDataSetPath, fmt.Sprintf("forest_%s_%s", hash, "tmp_predict"), xx, true)
	if err != nil {
		log.Error().Err(err).Msg("could not create Dataset file")
		return nil, r.metadata, err
	}

	if r.tree == nil {
		log.Error().Err(err).Msg("no tree trained")
		return nil, r.metadata, fmt.Errorf("no tree trained")
	}

	predictions, err := ml.RandomForestPredict(r.tree, fn, r.template, false)
	if err != nil {
		log.Error().Err(err).Msg("could not train with isolation forest")
		return nil, r.metadata, err
	}
	_, a := predictions.Size()
	lastPrediction := predictions.RowString(a - 1)
	return [][]float64{{fromEmoji(lastPrediction)}}, r.metadata, nil
}

func (r *RandomForest) Loss(actual, predicted [][]float64) []float64 {
	return SV(last(actual)).Diff(SV(last(predicted)))
}

func (r *RandomForest) Config() mlmodel.Model {
	return r.cfg
}

const benchmarkModelPath = "file-storage/ml/models"
const trainDataSetPath = "file-storage/ml/datasets"
const predictDataSetPath = "file-storage/ml/tmp"

func toFeatureFile(parentPath string, description string, vectors [][]float64, predict bool) (string, error) {
	fn, err := storage.MakePath(parentPath, fmt.Sprintf("%s.csv", description))
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
		for i, v := range vector {
			if i >= len(vector)-1 {
				lw.WriteString(fmt.Sprintf("%s", toEmoji(v)))
			} else {
				lw.WriteString(fmt.Sprintf("%.4f,", v))
			}
		}
		_, _ = writer.WriteString(lw.String() + "\n")
	}
	return fn, nil
}
