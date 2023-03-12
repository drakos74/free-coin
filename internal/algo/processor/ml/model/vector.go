package model

import (
	"fmt"
	"strings"

	"github.com/drakos74/free-coin/internal/model"
)

type Meta struct {
	Key    model.Key  `json:"key"`
	Tick   model.Tick `json:"tick"`
	Active bool       `json:"active"`
}

type Vector struct {
	Meta    Meta      `json:"meta"`
	PrevIn  []float64 `json:"prev_in"`
	PrevOut []float64 `json:"prev_out"`
	NewIn   []float64 `json:"new_in"`
}

func (v Vector) String() string {
	lw := new(strings.Builder)
	for _, in := range v.PrevIn {
		lw.WriteString(fmt.Sprintf("%f,", in))
	}
	if v.PrevOut[0] == 1.0 {
		lw.WriteString(fmt.Sprintf("%s", model.Buy.String()))
	} else if v.PrevOut[2] == 1.0 {
		lw.WriteString(fmt.Sprintf("%s", model.Sell.String()))
	} else {
		lw.WriteString(fmt.Sprintf("%s", model.NoType.String()))
	}
	return lw.String()
}
