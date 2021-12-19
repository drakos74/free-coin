package ml

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/sjwhitworth/golearn/base"
	"github.com/sjwhitworth/golearn/evaluation"
	"github.com/sjwhitworth/golearn/knn"
)

// KnnModel applies a knn model to our dataset
/**
5.1,3.5,1.4,0.2,Iris-setosa
4.9,3.0,1.4,0.2,Iris-setosa
...
*/
func KnnModel(file string) error {
	rawData, err := base.ParseCSVToInstances(file, false)
	if err != nil {
		panic(err)
	}

	//Initialises a new KNN classifier
	cls := knn.NewKnnClassifier("cosine", "linear", 10)

	//Do a training-test split
	trainData, testData := base.InstancesTrainTestSplit(rawData, 0.50)
	err = cls.Fit(trainData)
	if err != nil {
		log.Error().Err(err).Msg("could not train knn model")
		return err
	}

	//Calculates the Euclidean distance and returns the most popular label
	predictions, err := cls.Predict(testData)
	if err != nil {
		log.Error().Err(err).Msg("could not predict on knn model")
		return err
	}
	fmt.Println(predictions)

	// Prints precision/recall metrics
	confusionMat, err := evaluation.GetConfusionMatrix(testData, predictions)
	if err != nil {
		log.Error().Err(err).Msg("could not get confusion matrix")
		return err
	}
	fmt.Println(evaluation.GetSummary(confusionMat))
	return nil
}
