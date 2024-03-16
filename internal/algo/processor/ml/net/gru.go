package net

import (
	"fmt"
	"os"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/math/ml"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
)

const GRU_KEY string = "net.GRU"

type GRU struct {
	network  *ml.GRU
	config   mlmodel.Model
	metadata ml.Metadata
}

func NewGRU(config mlmodel.Model) *GRU {
	gru := ml.NewGRU(config.Size[0], config.Size[1], config.Size[2])
	return &GRU{
		network:  gru,
		config:   config,
		metadata: ml.NewMetadata(),
	}
}

func (gru *GRU) Train(x [][]float64, y [][]float64) (ml.Metadata, error) {
	in, out := strip(x, y)
	epochs, err := gru.network.Train(gru.config.LearningRate,
		gru.config.Threshold,
		gru.config.MaxEpochs,
		in,
		out)
	gru.metadata.Samples += epochs
	gru.metadata.Loss = err
	return gru.metadata, nil
}

func (gru *GRU) Predict(x [][]float64) ([][]float64, ml.Metadata, error) {
	out, _ := gru.network.Step(x[len(x)-1], make([]float64, gru.config.Size[1]))
	return [][]float64{quantifyAll(out, gru.config.Spread)}, gru.metadata, nil
}

func (gru *GRU) Loss(actual, predicted [][]float64) []float64 {
	lastActual := quantifyAll(last(actual), gru.config.Spread)
	lastPredicted := last(predicted)
	return SV(lastActual).Diff(SV(lastPredicted))
}

func (gru *GRU) Config() mlmodel.Model {
	return gru.config
}

func (gru *GRU) Load(key model.Key, detail mlmodel.Detail) error {
	dir := fmt.Sprintf("%s/net/%s", storage.DefaultDir, key.ToString())
	filename := fmt.Sprintf("%s/%s.json", dir, detail.ToString())
	return gru.network.Load(filename)
}

func (gru *GRU) Save(key model.Key, detail mlmodel.Detail) error {
	// create file name
	dir := fmt.Sprintf("%s/net/%s", storage.DefaultDir, key.ToString())
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create directory for model: %w", err)
	}
	filename := fmt.Sprintf("%s/%s.json", dir, detail.ToString())
	return gru.network.Save(filename)
}
