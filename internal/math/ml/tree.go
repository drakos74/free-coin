package ml

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/sjwhitworth/golearn/base"
	"github.com/sjwhitworth/golearn/ensemble"
	"github.com/sjwhitworth/golearn/evaluation"
	"github.com/sjwhitworth/golearn/filters"
)

func NewRandomForest(size, features int) *ensemble.RandomForest {
	return ensemble.NewRandomForest(size, features)
}

func PreProcessAttributes(iris *base.DenseInstances) (*base.LazilyFilteredInstances, error) {
	// Discretise the iris dataset with Chi-Merge
	filt := filters.NewChiMergeFilter(iris, 0.999)
	for _, a := range base.NonClassFloatAttributes(iris) {
		filt.AddAttribute(a)
	}
	err := filt.Train()
	if err != nil {
		return nil, err
	}
	irisf := base.NewLazilyFilteredInstances(iris, filt)
	return irisf, nil
}

func RandomForestTrain(tree base.Classifier, fileName string, size, features int, debug bool) (base.Classifier, *base.DenseInstances, float64, error) {
	rand.Seed(time.Now().Unix())

	// Load in the iris dataset
	iris, err := base.ParseCSVToInstances(fileName, false)
	if err != nil {
		return nil, nil, 0.0, err
	}

	irisf, err := PreProcessAttributes(iris)
	if err != nil {
		return nil, nil, 0.0, err
	}

	// Create a 60-40 training-test split
	trainData, testData := base.InstancesTrainTestSplit(irisf, 0.60)

	if tree == nil {
		tree = NewRandomForest(size, features)
	}
	err = tree.Fit(trainData)
	if err != nil {
		return nil, nil, 0.0, err
	}
	predictions, err := tree.Predict(testData)
	if err != nil {
		return nil, nil, 0.0, err
	}

	cf, err := evaluation.GetConfusionMatrix(testData, predictions)
	if err != nil {
		return nil, nil, 0.0, err
	}

	if debug {
		fmt.Println("RandomForest Performance")
		fmt.Println(evaluation.GetSummary(cf))
	}

	return tree, iris, evaluation.GetAccuracy(cf), nil
}

func RandomForestPredict(tree base.Classifier, fileName string, debug bool) (base.FixedDataGrid, error) {

	// Load in the dataset
	ds, err := base.ParseCSVToInstances(fileName, false)
	if err != nil {
		return nil, err
	}

	dsf, err := PreProcessAttributes(ds)
	if err != nil {
		return nil, err
	}

	err = tree.Fit(dsf)
	if err != nil {
		return nil, err
	}

	predictions, err := tree.Predict(dsf)
	if err != nil {
		return nil, err
	}

	return predictions, nil
}
