package ml

import (
	"fmt"
	"math"

	"github.com/cdipaolo/goml/cluster"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/free-coin/internal/storage/file/json"
	"github.com/rs/zerolog/log"
)

type KMeans struct {
	dataKey    storage.Key
	resultsKey storage.Key
	dim        int
	iterations int
	model      *cluster.KMeans
	stats      map[int]*buffer.Stats
	store      storage.Persistence
}

func NewKMeans(pair string, dim int, iterations int) *KMeans {
	// overwrite
	dataKey := storage.Key{
		Pair:  pair,
		Label: "data",
	}
	resultKey := storage.Key{
		Pair:  pair,
		Label: "results",
	}
	stats := make(map[int]*buffer.Stats, dim)
	return &KMeans{
		dataKey:    dataKey,
		resultsKey: resultKey,
		stats:      stats,
		dim:        dim,
		iterations: iterations,
		store:      json.NewJsonBlob("ml", "kmeans", false),
	}
}

type Stats struct {
	Size int
	Avg  float64
}

func transform(stats map[int]*buffer.Stats) map[int]Stats {
	newStats := make(map[int]Stats)
	for g, st := range stats {
		newStats[g] = Stats{
			Size: st.Count(),
			Avg:  st.Avg(),
		}
	}
	return newStats
}

func (k *KMeans) Train(x []float64, y float64, train bool) (map[int]Stats, error) {
	stats := make(map[int]Stats)
	// load all data
	var data [][]float64
	if err := k.store.Load(k.dataKey, &data); err != nil {
		log.Warn().Err(err).
			Str("pair", k.dataKey.Pair).
			Str("label", k.dataKey.Label).
			Int64("hash", k.dataKey.Hash).
			Msg("no previous data for k-means")
	}
	// append new data to data
	data = append(data, x)
	// load results
	var results []float64
	if err := k.store.Load(k.resultsKey, &results); err != nil {
		log.Warn().Err(err).
			Str("pair", k.resultsKey.Pair).
			Str("label", k.resultsKey.Label).
			Int64("hash", k.resultsKey.Hash).
			Msg("no previous results for k-means")
	}
	results = append(results, y)
	// store back
	if err := k.store.Store(k.dataKey, data); err != nil {
		log.Error().
			Err(err).
			Str("key", fmt.Sprintf("%+v", k.dataKey)).
			Int("data", len(data)).
			Msg("could not store data set for k-means")
		return stats, fmt.Errorf("could not store updated data set: %w", err)
	}
	if err := k.store.Store(k.resultsKey, results); err != nil {
		log.Error().
			Err(err).
			Str("key", fmt.Sprintf("%+v", k.resultsKey)).
			Int("results", len(results)).
			Msg("could not store result set for k-means")
		return stats, fmt.Errorf("could not store updated results set: %w", err)
	}
	// train either on demand or based on intervals
	if train && (len(data) >= k.dim) {
		k.model = cluster.NewKMeans(k.dim, k.iterations, data)
		if err := k.model.Learn(); err != nil {
			log.Error().
				Err(err).
				Str("key", fmt.Sprintf("%+v", k.dataKey)).
				Msg("error during training on k-means")
			return stats, fmt.Errorf("could not train: %w", err)
		}
		guesses := k.model.Guesses()
		if len(guesses) != len(results) {
			return stats, fmt.Errorf("could not align results with data [ %d | %d | %d ]", len(results), len(guesses), len(data))
		}
		//calculate score for each of the clusters
		k.stats = make(map[int]*buffer.Stats, k.dim)
		for i := 0; i < len(guesses); i++ {
			g := guesses[i]
			if _, ok := k.stats[g]; !ok {
				k.stats[g] = buffer.NewStats()
			}
			k.stats[g].Push(results[i])
		}
		stats = transform(k.stats)
	}
	return stats, nil
}

func (k *KMeans) Predict(x []float64) (int, float64, map[int]Stats, error) {
	if k.model == nil {
		return 0, 0, map[int]Stats{}, fmt.Errorf("no model present")
	}
	guess, err := k.model.Predict(x)
	if err != nil {
		log.Error().
			Err(err).
			Str("key", fmt.Sprintf("%+v", k.dataKey)).
			Msg("could not predict for k-means")
		return 0, 0, map[int]Stats{}, fmt.Errorf("could not predict: %w", err)
	}

	f := int(math.Round(guess[0]))
	score := k.stats[f].Avg()

	ss := transform(k.stats)

	return f, score, ss, nil
}
