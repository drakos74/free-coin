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
