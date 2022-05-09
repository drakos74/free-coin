package net

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/client"

	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

type Dataset struct {
	Coin     model.Coin
	Duration time.Duration
	Vectors  []mlmodel.Vector
	config   mlmodel.Model
	Network  *MultiNetwork
}

func (s Dataset) getDescription(postfix string) string {
	return fmt.Sprintf("%s_%s_%.2f_%s", s.Coin, s.Duration, s.config.PrecisionThreshold, postfix)
}

func (s *Dataset) Train() (ModelResult, map[mlmodel.Detail]ModelResult) {
	return s.Network.Train(s)
}

func (s *Dataset) Eval(k mlmodel.Detail, report client.Report) {
	s.Network.Eval(k, report)
}

type Datasets struct {
	sets    map[model.Key]*Dataset
	storage storage.Persistence
	network ConstructMultiNetwork
}

func NewDataSets(shard storage.Shard, network ConstructMultiNetwork) *Datasets {
	persistence, err := shard("vectors")
	if err != nil {
		log.Error().Err(err).Msg("could not crete vector storage")
		persistence = storage.VoidStorage{}
	}
	return &Datasets{
		sets:    make(map[model.Key]*Dataset),
		storage: persistence,
		network: network,
	}
}

func (ds *Datasets) Sets() map[model.Key]*Dataset {
	return ds.sets
}

func (ds *Datasets) saveVectors(key model.Key, vectors []mlmodel.Vector) error {
	return ds.storage.Store(stKey(key), vectors)
}

func (ds *Datasets) loadVectors(key model.Key) ([]mlmodel.Vector, error) {
	vectors := make([]mlmodel.Vector, 0)
	err := ds.storage.Load(stKey(key), &vectors)
	return vectors, err
}

func stKey(key model.Key) storage.Key {
	return storage.Key{
		Pair:  fmt.Sprintf("%v_%v", key.Coin, key.Duration),
		Hash:  key.Index,
		Label: key.Strategy,
	}
}

func (ds *Datasets) Push(key model.Key, vv mlmodel.Vector, cfg mlmodel.Model) (*Dataset, bool) {
	if _, ok := ds.sets[key]; !ok {
		vectors := make([]mlmodel.Vector, 0)
		vv, err := ds.loadVectors(key)
		if err == nil {
			vectors = vv
			log.Info().Str("Index", key.ToString()).Int("vv", len(vv)).Msg("loaded vectors")
		} else {
			log.Error().Err(err).Str("Index", key.ToString()).Msg("could not load vectors")
		}
		ds.sets[key] = newDataSet(key.Coin, key.Duration, cfg, vectors, ds.network(cfg))
	}
	// keep only the last Vectors based on the buffer size + 10% to cover for the Max
	bufferSize := mlmodel.EvolveInt(cfg.BufferSize, 1.0)
	newVectors := addVector(ds.sets[key].Vectors, vv, bufferSize)
	err := ds.saveVectors(key, newVectors)
	if err != nil {
		log.Error().
			Err(err).
			Str("vectors", fmt.Sprintf("%+v", newVectors)).
			Str("Index", key.ToString()).
			Msg("could not save vectors")
	}

	ds.sets[key] = newDataSet(key.Coin, key.Duration, cfg, newVectors, ds.sets[key].Network)

	if len(ds.sets[key].Vectors) >= bufferSize {
		return ds.sets[key], true
	}
	return &Dataset{}, false
}

func addVector(ss []mlmodel.Vector, s mlmodel.Vector, size int) []mlmodel.Vector {
	newVectors := append(ss, s)
	l := len(newVectors)
	if l > size {
		newVectors = newVectors[l-size:]
	}
	return newVectors
}

func newDataSet(coin model.Coin, duration time.Duration, cfg mlmodel.Model, vv []mlmodel.Vector, network *MultiNetwork) *Dataset {
	return &Dataset{
		Coin:     coin,
		Duration: duration,
		Vectors:  vv,
		config:   cfg,
		Network:  network,
	}
}
