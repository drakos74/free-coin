package ml

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/sjwhitworth/golearn/base"
	"github.com/sjwhitworth/golearn/ensemble"
	"github.com/sjwhitworth/golearn/evaluation"
	"github.com/sjwhitworth/golearn/filters"
	"github.com/sjwhitworth/golearn/trees"
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
	if tree == nil {
		tree = NewRandomForest(size, features)
	}

	// Load in the iris dataset
	iris, err := base.ParseCSVToInstances(fileName, false)
	if err != nil {
		return nil, nil, 0.0, fmt.Errorf("could not parse csv: %w", err)
	}

	irisf, err := PreProcessAttributes(iris)
	if err != nil {
		return nil, nil, 0.0, fmt.Errorf("could not pre-process: %w", err)
	}

	// make it an ID3DecisionTree
	tree = trees.NewID3DecisionTree(0.6)

	// Create a 60-40 training-test split
	trainData, testData := base.InstancesTrainTestSplit(irisf, 0.60)

	err = tree.Fit(trainData)
	if err != nil {
		return nil, nil, 0.0, fmt.Errorf("could not fit: %w", err)
	}
	predictions, err := tree.Predict(testData)
	if err != nil {
		return nil, nil, 0.0, fmt.Errorf("could not predict: %w", err)
	}

	cf, err := evaluation.GetConfusionMatrix(testData, predictions)
	if err != nil {
		return nil, nil, 0.0, fmt.Errorf("could not get confusion matrix: %w", err)
	}

	if debug {
		fmt.Println("RandomForest Performance")
		fmt.Println(evaluation.GetSummary(cf))
	}

	return tree, iris, evaluation.GetAccuracy(cf), nil
}

func RandomForestPredict(tree base.Classifier, fileName string, template *base.DenseInstances, debug bool) (base.FixedDataGrid, error) {

	// Load in the dataset
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ds, err := base.ParseCSVToTemplatedInstancesFromReader(f, false, template)
	//ds, err := base.ParseCSVToInstances(fileName, false)
	if err != nil {
		return nil, err
	}

	iris, err := PreProcessAttributes(ds)
	if err != nil {
		return nil, err
	}

	predictions, err := tree.Predict(iris)
	if err != nil {
		return nil, err
	}

	return predictions, nil
}
