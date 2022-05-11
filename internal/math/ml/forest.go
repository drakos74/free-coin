package ml

import (
	"fmt"
	"math/rand"

	"github.com/rs/zerolog/log"

	randomforest "github.com/malaschitz/randomForest"
)

type RandomForest struct {
	trees  int
	forest *randomforest.Forest
}

func NewForest(n int) *RandomForest {
	return &RandomForest{
		trees: n,
	}
}

func (rf *RandomForest) Train(xData [][]float64, yData []int) (float64, []float64) {
	forest := &randomforest.Forest{}
	log.Info().Str("xData", fmt.Sprintf("%+v", xData)).Str("yData", fmt.Sprintf("%+v", yData)).Msg("data")
	forest.Data = randomforest.ForestData{X: xData, Class: yData}
	//dForest := forest.BuildDeepForest()
	//dForest.Train(20, 100, 1000)
	forest.Train(rf.trees)
	rf.forest = forest
	features := forest.FeatureImportance
	//selectedFeatures, allFeatures := randomforest.BorutaDefault(xData, yData)
	//fmt.Printf("selectedFeatures = %+v\n", selectedFeatures)
	//fmt.Printf("allFeatures = %+v\n", allFeatures)
	return 1.0, features
}

func (rf *RandomForest) Predict(xData []float64) []float64 {
	ff := rf.forest.Vote(xData)
	return ff
}

func TrainForest() {
	var xData [][]float64
	var yData []int
	for i := 0; i < 1000; i++ {
		x := []float64{rand.Float64(), rand.Float64(), rand.Float64(), rand.Float64()}
		//x := []float64{float64(i), float64(i % 10), float64(i % 100), float64(i % 1000)}
		y := int(x[0] + x[1] + x[2] + x[3])
		xData = append(xData, x)
		yData = append(yData, y)
	}
	forest := randomforest.Forest{}
	forest.Data = randomforest.ForestData{X: xData, Class: yData}
	forest.Train(1000)
	//test
	fmt.Println("Vote", forest.Vote([]float64{0.1, 0.1, 0.1, 0.1}))
	fmt.Println("Vote", forest.Vote([]float64{0.9, 0.9, 0.9, 0.9}))
}
