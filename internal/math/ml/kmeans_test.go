package ml

import (
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cdipaolo/goml/cluster"
)

func TestKMeans(t *testing.T) {
	var double [][]float64
	var score []float64
	for i := -10.0; i < -3; i += 1.0 {
		for j := -10.0; j < 10; j += 1.0 {
			double = append(double, []float64{i, j})
			score = append(score, rand.Float64())
		}
	}
	for i := 3.0; i < 10; i += 1.0 {
		for j := -10.0; j < 10; j += 1.0 {
			double = append(double, []float64{i, j})
			score = append(score, -1*rand.Float64())
		}
	}
	fmt.Printf("double = %+v\n", double)
	fmt.Printf("score = %+v\n", score)

	e1 := os.Remove("file-storage/ml/kmeans/pair_0_data.json")
	assert.NoError(t, e1)
	e2 := os.Remove("file-storage/ml/kmeans/pair_0_results.json")
	assert.NoError(t, e2)

	kmeans := NewKMeans("pair", 4, 30)

	for i := 0; i < len(double); i++ {
		d := double[i]
		meta, err := kmeans.Train(d, score[i], false)
		fmt.Printf("meta = %+v\n", meta)
		assert.NoError(t, err)
		if i == len(double)-1 {
			_, err := kmeans.Train(d, score[i], true)
			assert.NoError(t, err)
		}
	}

	c1, confidence, ss, err := kmeans.Predict([]float64{-7.5, 0}, 2)
	fmt.Printf("ss = %+v\n", ss)
	assert.NoError(t, err)
	fmt.Printf("c1 = %+v | %+v | %+v \n", c1, confidence, ss)

	c2, confidence, ss, err := kmeans.Predict([]float64{7.5, 0}, 2)
	fmt.Printf("ss = %+v\n", ss)
	assert.NoError(t, err)
	fmt.Printf("c2 = %+v | %+v| %+v \n", c2, confidence, ss)
}

func TestTrainKMeans(t *testing.T) {

	double := [][]float64{}
	for i := -10.0; i < -3; i += 1.0 {
		for j := -10.0; j < 10; j += 1.0 {
			double = append(double, []float64{i, j})
		}
	}
	for i := 3.0; i < 10; i += 1.0 {
		for j := -10.0; j < 10; j += 1.0 {
			double = append(double, []float64{i, j})
		}
	}
	fmt.Printf("double = %+v\n", double)
	model := cluster.NewKMeans(2, 30, double)
	if model.Learn() != nil {
		panic("Oh NO!!! There was an error learning!!")
	}
	// now predict with the same training set and
	// make sure the classes are the same within
	// each block
	c1, err := model.Predict([]float64{-7.5, 0})
	if err != nil {
		panic("prediction error")
	}
	fmt.Printf("c1 = %+v\n", c1)
	c2, err := model.Predict([]float64{7.5, 0})
	if err != nil {
		panic("prediction error")
	}
	fmt.Printf("c2 = %+v\n", c2)
	// now you can predict like normal!
	guess, err := model.Predict([]float64{-3, 6})
	fmt.Printf("guess = %+v\n", guess)
	if err != nil {
		panic("prediction error")
	}
	// or if you want to get the clustering
	// results from the data
	results := model.Guesses()
	fmt.Printf("results = %+v\n", results)
	// you can also concat that with the
	// training set and save it to a file
	// (if you wanted to plot it or something)
	err = model.SaveClusteredData("/tmp/.goml/KMeansResults.csv")
	if err != nil {
		panic("file save error")
	}
	// you can also persist the model to a
	// file
	err = model.PersistToFile("/tmp/.goml/KMeans.json")
	if err != nil {
		panic("file save error")
	}
	// and also restore from file (at a
	// later time if you want)
	err = model.RestoreFromFile("/tmp/.goml/KMeans.json")
	if err != nil {
		panic("file save error")
	}
}
