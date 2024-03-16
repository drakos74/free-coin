package net

import (
	"fmt"
	"os"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/emoji"
	"github.com/drakos74/free-coin/internal/math/ml"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
)

const HMM_KEY string = "net.HMM"

type HMM struct {
	net      *buffer.HMM
	cfg      mlmodel.Model
	metadata ml.Metadata
}

func NewHMM(cfg mlmodel.Model) *HMM {
	return &HMM{
		net:      buffer.NewMultiHMM(buffer.NewHMMConfig(cfg.Features[0], cfg.Features[1], emoji.DotSnow)),
		cfg:      cfg,
		metadata: ml.NewMetadata(),
	}
}

func (hmm *HMM) Train(x [][]float64, y [][]float64) (ml.Metadata, error) {
	status := hmm.net.Add(toEmoji(lastAt(0, y), hmm.cfg.Spread))
	hmm.metadata.Samples = status.Count
	return hmm.metadata, nil
}

func (hmm *HMM) Predict(x [][]float64) ([][]float64, ml.Metadata, error) {
	input := buffer.NewSequence(toEmojis(toSeries(0, x, nil), hmm.cfg.Spread))
	predictions := hmm.net.Predict(input)
	maxProb := 0.0
	value := buffer.NewSequence([]string{})
	for _, prediction := range predictions {
		if prediction.Sample > hmm.cfg.BufferSize {
			list := prediction.Values
			// predictions are already sorted
			best := list[0]
			if best.Probability > maxProb {
				value = best.Value
			}
		}
	}
	// convert to numbers and converge to single output
	return converge(fromEmojis(value.Values())), hmm.metadata, nil
}

func (hmm *HMM) Loss(actual, predicted [][]float64) []float64 {
	lastActual := fromEmojis(toEmojis(actual[len(actual)-1], hmm.cfg.Spread))
	lastPredicted := last(predicted)
	return SV(lastActual).Diff(SV(lastPredicted))
}

func (hmm *HMM) Config() mlmodel.Model {
	return hmm.cfg
}

func (hmm *HMM) Load(key model.Key, detail mlmodel.Detail) error {
	dir := fmt.Sprintf("%s/net/%s", storage.DefaultDir, key.ToString())
	filename := fmt.Sprintf("%s/%s.json", dir, detail.ToString())
	return hmm.net.Load(filename)
}

func (hmm *HMM) Save(key model.Key, detail mlmodel.Detail) error {
	// create file name
	dir := fmt.Sprintf("%s/net/%s", storage.DefaultDir, key.ToString())
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create directory for model: %w", err)
	}
	filename := fmt.Sprintf("%s/%s.json", dir, detail.ToString())
	return hmm.net.Save(filename)
}
