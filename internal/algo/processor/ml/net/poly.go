package net

import (
	"fmt"
	math2 "math"

	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/math"
	"github.com/drakos74/free-coin/internal/math/ml"
)

const POLY_KEY string = "net.Polynomial"

type Polynomial struct {
	order    int
	next     int
	cfg      mlmodel.Model
	buffer   *buffer.Buffer
	metadata ml.Metadata
}

func NewPolynomial(cfg mlmodel.Model) *Polynomial {
	if cfg.BufferSize <= 0 {
		panic(fmt.Errorf("cannot init properly polynomial network with bad config [BufferSize > 0] : [%+v]", cfg.BufferSize))
	}
	if len(cfg.Features) < 2 {
		panic(fmt.Errorf("cannot init properly polynomial network with bad config [features.len > 0] : [%+v]", cfg.Features))
	}
	return &Polynomial{
		cfg:      cfg,
		buffer:   buffer.NewBuffer(cfg.BufferSize),
		order:    cfg.Features[0],
		next:     cfg.Features[1],
		metadata: ml.NewMetadata(),
	}
}

func (p *Polynomial) Train(x [][]float64, y [][]float64) (ml.Metadata, error) {
	// pick the first attribute (which we reasonably assume to be the price trend)
	if _, ok := p.buffer.Push(last(y)[0]); ok {
		return p.metadata, nil
	}
	return p.metadata, fmt.Errorf("buffer not filled: %d of %d", p.buffer.Size(), p.cfg.BufferSize)
}

func (p *Polynomial) Predict(x [][]float64) ([][]float64, ml.Metadata, error) {
	xx := make([]float64, p.cfg.BufferSize+1)
	yy := make([]float64, p.cfg.BufferSize+1)
	bb := p.buffer.GetAsFloats(false)
	l := len(bb)
	for i, b := range bb {
		xx[i] = float64(i)
		yy[i] = b
	}
	xx[l] = float64(l)
	yy[l] = lastAt(0, x)
	a, err := math.Fit(xx, yy, p.order)
	if err != nil {
		return nil, p.metadata, fmt.Errorf("error during fit: %w", err)
	}

	r := make([]float64, p.next)
	for n := 0; n < p.next; n++ {
		x0 := float64(len(xx) + n)
		y := a[0]
		for i := range a {
			y += a[i] * math2.Pow(x0, float64(i))
		}
		r[n] = quantify(y, p.cfg.Spread)
	}

	return [][]float64{{sameOrNothing(r)}}, p.metadata, nil
}

func (p *Polynomial) Loss(actual, predicted [][]float64) []float64 {
	lastActual := quantifyAll(last(actual), p.cfg.Spread)
	lastPredicted := last(predicted)
	return SV(lastActual).Diff(SV(lastPredicted))
}

func (p *Polynomial) Config() mlmodel.Model {
	return p.cfg
}

func (p *Polynomial) Load(key model.Key, detail mlmodel.Detail) error {
	log.Debug().Msg("nothing to load for polynomial network")
	return nil
}

func (p *Polynomial) Save(key model.Key, detail mlmodel.Detail) error {
	log.Debug().Msg("nothing to save for polynomial network")
	return nil
}
