package ml

import (
	"github.com/drakos74/free-coin/internal/storage"
)

type Dataset struct {
	data  map[int][][]float64
	file  string
	store storage.Persistence
	key   storage.Key
}

type Metadata struct {
	Samples int
	Stats   map[int]Stats
	Scores  []float64
	Limit   float64
}

type Stats struct {
	Size int
	Avg  float64
}
