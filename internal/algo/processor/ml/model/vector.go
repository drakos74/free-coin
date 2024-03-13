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
	lw.WriteString(fmt.Sprintf("%+v", v.Meta))
	lw.WriteString("\npreI:")
	for _, in := range v.PrevIn {
		lw.WriteString(fmt.Sprintf("%f,", in))
	}
	lw.WriteString("\npreO:")
	for _, in := range v.PrevOut {
		lw.WriteString(fmt.Sprintf("%f,", in))
	}
	lw.WriteString("\nnewI:")
	for _, in := range v.NewIn {
		lw.WriteString(fmt.Sprintf("%f,", in))
	}
	return lw.String()
}
