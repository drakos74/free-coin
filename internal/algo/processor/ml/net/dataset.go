package net

import (
	"fmt"
	"time"

	"github.com/drakos74/free-coin/client"
	mlmodel "github.com/drakos74/free-coin/internal/algo/processor/ml/model"
	"github.com/drakos74/free-coin/internal/math/ml"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

var (
	allCoinsKey = model.Key{
		Coin: model.AllCoins,
	}
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
	sets      map[model.Key]*Dataset
	decisions map[model.Key]Model
	storage   storage.Persistence
	network   ConstructMultiNetwork
}

func NewDataSets(shard storage.Shard, network ConstructMultiNetwork) *Datasets {
	persistence, err := shard("vectors")
	if err != nil {
		log.Error().Err(err).Msg("could not crete vector storage")
		persistence = storage.VoidStorage{}
	}
	return &Datasets{
		sets: make(map[model.Key]*Dataset),
		decisions: map[model.Key]Model{
			allCoinsKey: ml.NewKMeans("all", 5, 30),
		},
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

func (ds *Datasets) Eval(key model.Key, x []float64, leadingThreshold int) (int, float64, ml.Metadata, error) {
	if _, ok := ds.decisions[key]; !ok {
		log.Warn().Str("method", "eval").Str("key", fmt.Sprintf("%+v", key)).Msg("new decision record for key")
		ds.decisions[key] = ml.NewKMeans(string(key.Coin), 5, 30)
	}
	cl, score, meta, err := ds.decisions[key].Predict(x, leadingThreshold)
	if err != nil {
		log.Warn().Err(err).Str("key", fmt.Sprintf("%+v", key)).Msg("eval model fallback")
		return ds.decisions[allCoinsKey].Predict(x, leadingThreshold)
	}
	return cl, score, meta, nil
}

func (ds *Datasets) Cluster(key model.Key, x []float64, y float64, train bool) (ml.Metadata, error) {
	if _, ok := ds.decisions[key]; !ok {
		log.Warn().Str("method", "cluster").Str("key", fmt.Sprintf("%+v", key)).Msg("new decision record for key")
		ds.decisions[key] = ml.NewKMeans(string(key.Coin), 5, 30)
	}
	meta, err := ds.decisions[key].Train(x, y, train)
	if err != nil {
		log.Warn().Err(err).Str("key", fmt.Sprintf("%+v", key)).Msg("cluster model fallback")
		return ds.decisions[allCoinsKey].Train(x, y, train)
	}
	return meta, nil
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
