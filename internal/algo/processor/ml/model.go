package ml

import (
	"time"

	"github.com/sjwhitworth/golearn/base"

	"github.com/drakos74/free-coin/internal/model"
)

// Config defines the configuration for the collector.
type Config map[model.Coin]map[time.Duration]Segments

// Segments defines the look back and ahead segment number.
type Segments struct {
	LookBack  int
	LookAhead int
	Threshold float64
	Model     string
	MLModel   base.Classifier
	MlDataSet *base.DenseInstances
}

// Signal represents a signal from the ml processor.
type Signal struct {
	Coin      model.Coin    `json:"coin"`
	Time      time.Time     `json:"time"`
	Duration  time.Duration `json:"duration"`
	Price     float64       `json:"price"`
	Type      model.Type    `json:"type"`
	Precision float64       `json:"precision"`
}
