package ml

type Metadata struct {
	Samples  int
	Clusters map[int]Cluster
	Features []float64
	Accuracy float64
	Loss     []float64
}

func NewMetadata() Metadata {
	return Metadata{
		Clusters: make(map[int]Cluster),
		Features: make([]float64, 0),
		Loss:     make([]float64, 0),
	}
}

type Cluster struct {
	Size int
	Avg  float64
}
