package net

import (
	"fmt"
	math2 "math"

	"github.com/drakos74/go-ex-machina/xmath"

	"github.com/drakos74/free-coin/internal/emoji"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/model"
)

// DataSet is a single training segment of several input and output tensors
type DataSet struct {
	Key     model.Key
	in      int
	out     int
	In      [][]float64
	Out     [][]float64
	PrevOut [][]float64
}

func NewDataSet(in, out int) DataSet {
	return DataSet{
		in:  in,
		out: out,
		In:  make([][]float64, 0),
		Out: make([][]float64, 0),
	}
}

func (ds *DataSet) String() string {
	return fmt.Sprintf("%v->%v\nin :%+v\nout:%+v\n",
		ds.in, ds.out,
		ds.In,
		ds.Out)
}

func (ds *DataSet) Push(k model.Key, vv mlmodel.Vector) ([][]float64, bool, error) {
	if ds.Key.Coin != "" {
		if k.Hash() != ds.Key.Hash() {
			return nil, false, fmt.Errorf("wrong key (%+v) for tensor (%+v)", k, ds.Key)
		}
	}

	filled := false

	ds.In = append(ds.In, vv.PrevIn)
	if len(ds.In) > ds.in {
		ds.In = ds.In[1:]
		filled = true
	}

	ds.Out = append(ds.Out, vv.PrevOut)
	if len(ds.Out) > ds.out {
		ds.Out = ds.Out[1:]
	} else {
		filled = false
	}

	input := append(ds.In, vv.NewIn)

	return input, filled, nil
}

func toSeries(index int, x, y [][]float64) []float64 {
	series := make([]float64, len(x))
	for i := range x {
		series[i] = x[i][index]
	}
	if y != nil {
		series = append(series, y[len(y)-1][index])
	}
	return series
}

func lastAt(index int, y [][]float64) float64 {
	return y[len(y)-1][index]
}

func last(y [][]float64) []float64 {
	return y[len(y)-1]
}

func strip(in, out [][]float64) ([][]float64, [][]float64) {
	l := int(math2.Min(float64(len(in)), float64(len(out))))
	return in[len(in)-l:], out[len(out)-l:]
}

func toEmoji(v float64) string {
	if v > 0.5 {
		return emoji.DotFire
	} else if v < -0.5 {
		return emoji.DotWater
	}
	return emoji.DotSnow
}

func fromEmoji(s string) float64 {
	switch s {
	case emoji.DotSnow:
		return 0
	case emoji.DotFire:
		return 1
	case emoji.DotWater:
		return -1
	}
	return 0
}

func toEmojis(v []float64) []string {
	s := make([]string, len(v))
	for i, _ := range v {
		s[i] = toEmoji(v[i])
	}
	return s
}

func fromEmojis(s []string) []float64 {
	v := make([]float64, len(s))
	for i, _ := range s {
		v[i] = fromEmoji(s[i])
	}
	return v
}

func V(v []float64) xmath.Vector {
	return xmath.Vec(len(v)).With(v...)
}

func SV(v []float64) xmath.Vector {
	return V([]float64{v[0]})
}

func sameOrNothing(v []float64) float64 {
	value := v[0]
	for i := range v {
		if v[i] != value {
			return 0
		}
	}
	return value
}

func converge(v []float64) [][]float64 {
	value := v[0]
	for i := range v {
		if v[i] != value {
			return [][]float64{{0}}
		}
	}
	return [][]float64{{value}}
}
