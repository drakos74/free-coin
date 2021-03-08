package trade

import (
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
)

func triggerKey(coin string) storage.K {
	return storage.K{
		Pair:  coin,
		Label: ProcessorName,
	}
}

func strategyKey(coin string) storage.K {
	return storage.K{
		Pair:  coin,
		Label: "strategy",
	}
}

type PredictionPair struct {
	ID          string             `json:"id"`
	SignalID    string             `json:"signal"`
	Price       float64            `json:"price"`
	Time        time.Time          `json:"time"`
	Confidence  float64            `json:"confidence"`
	Strategy    processor.Strategy `json:"strategy"`
	Label       string             `json:"label"`
	Key         buffer.Sequence    `json:"key"`
	Values      []buffer.Sequence  `json:"values"`
	Probability float64            `json:"probability"`
	Sample      int                `json:"sample"`
	Type        model.Type         `json:"type"`
}

type predictionsPairs []PredictionPair

// for sorting predictions
func (p predictionsPairs) Len() int           { return len(p) }
func (p predictionsPairs) Less(i, j int) bool { return p[i].Probability < p[j].Probability }
func (p predictionsPairs) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type StrategyEvent struct {
	Coin        model.Coin  `json:"coin"`
	Time        time.Time   `json:"time"`
	Strategy    string      `json:"strategy"`
	Sample      Sample      `json:"sample"`
	Probability Probability `json:"probability"`
	Result      Result      `json:"result"`
}

type Sample struct {
	Strategy    int  `json:"strategy"`
	Predictions int  `json:"prediction"`
	Valid       bool `json:"valid"`
}

type Probability struct {
	Strategy    float64           `json:"strategy"`
	Predictions float64           `json:"prediction"`
	Valid       bool              `json:"valid"`
	Values      []buffer.Sequence `json:"values"`
}

type Result struct {
	Sum        float64 `json:"sum"`
	Count      float64 `json:"count"`
	Rating     float64 `json:"rating"`
	Type       string  `json:"type"`
	Threshold  float64 `json:"threshold"`
	Confidence float64 `json:"confidence"`
}
