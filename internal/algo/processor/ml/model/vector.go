package model

import "github.com/drakos74/free-coin/internal/model"

type Meta struct {
	Key  model.Key  `json:"key"`
	Tick model.Tick `json:"tick"`
}

type Vector struct {
	Meta    Meta      `json:"meta"`
	PrevIn  []float64 `json:"prev_in"`
	PrevOut []float64 `json:"prev_out"`
	NewIn   []float64 `json:"new_in"`
	XX      []float64 `json:"xx"`
	YY      []float64 `json:"yy"`
}
