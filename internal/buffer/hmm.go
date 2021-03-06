package buffer

import (
	"container/ring"
	"fmt"
	"sort"
	"strings"
	"time"

	cointime "github.com/drakos74/free-coin/internal/time"
)

const Delimiter = ":"

// HMMConfig defines the configuration for the hidden markov model analysis.
type HMMConfig struct {
	LookBack  int
	LookAhead int
}

// NewMultiHMM creates a new hmm.
func NewMultiHMM(config ...HMMConfig) *HMM {
	var max int
	for _, s := range config {
		if s.LookBack+s.LookAhead > max {
			max = s.LookBack + s.LookAhead
		}
	}
	return &HMM{
		max:      max,
		sequence: ring.New(max + 1),
		Config:   config,
		hmm:      make(map[Sequence]map[Sequence]int),
		Status:   newStatus(),
	}
}

// HMM counts occurrences in a sequence of strings.
// It implements effectively several hidden markov model of the n-grams lengths provided in the Config.
type HMM struct {
	max      int
	sequence *ring.Ring
	Config   []HMMConfig
	hmm      map[Sequence]map[Sequence]int
	Status   *Status
}

// Prediction defines a prediction result with the computed Probability
type Prediction struct {
	// ID is a unique numeric id related to the prediction
	ID int64
	// Value for the prediction . Essentially the concatenated string of the predicted sequence
	Value Sequence
	// Occur is the number of occurrences for the current combination.
	Occur int
	// Probability for the current prediction
	Probability float64
}

// Sequence defines a sequence of strings.
type Sequence string

// Values returns the hidden values of the sequence.
// We are doing this work-around to be able to use a slice of strings as a key in the Predictions map
func (s Sequence) Values() []string {
	return strings.Split(string(s), Delimiter)
}

// NewSequence creates a new sequence from a string
func NewSequence(s []string) Sequence {
	return Sequence(strings.Join(s, Delimiter))
}

type Predictions struct {
	Key Sequence
	// Values are the prediction details for each prediction
	Values PredictionList
	// Sample is the number of previous incidents of the source sequence that generated the current probability matrix
	Sample int
	// Groups is the number of groups / combinations of source sequences encountered of the given length.
	// TODO :assess the statistical significance of this
	Groups int
	// Count is the count of invocations for this model
	Count int
	// Label is a string acting as metadata for the prediction
	Label string
}

// TODO : maybe better to choose a uuid, for now the unix second should be enough
func NewPrediction(s Sequence, occur int) *Prediction {
	return &Prediction{
		ID:    cointime.ToNano(time.Now()),
		Value: s,
		Occur: occur,
	}
}

// PredictionList is a sortable list of predictions
type PredictionList []*Prediction

// for sorting predictions
func (p PredictionList) Len() int           { return len(p) }
func (p PredictionList) Less(i, j int) bool { return p[i].Occur < p[j].Occur }
func (p PredictionList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type Sample struct {
	Key    Sequence
	Events int
}

// Status reflects the current Status of the HMM
type Status struct {
	Count   int64
	Samples map[string]map[Sequence]Sample
}

func newStatus() *Status {
	return &Status{
		Samples: make(map[string]map[Sequence]Sample),
	}
}

// Add adds a string to the hmm sequence.
// TODO : reverse the logic by accepting the result instead of the input.
// (This should allow us to filter out irrelevant data adn save space,
// Note : hmm is expensive in terms of memory storage )
func (c *HMM) Add(s string, label string) (map[Sequence]Predictions, Status) {

	// We cant allow our delimiter char to be used within the values.
	if strings.Contains(s, Delimiter) {
		panic(fmt.Sprintf("illegal character found '%s' in '%s'", Delimiter, s))
	}

	c.Status.Count++

	// gather the previous values
	ring := make([]string, 0)
	c.sequence.Do(func(i interface{}) {
		ring = append(ring, fmt.Sprintf("%v", i))
	})

	prediction := make(map[Sequence]Predictions)

	for _, cfg := range c.Config {
		if k, predict, ss, cc := c.addKey(cfg, ring, s); len(predict.Values) > 0 {
			predict.Label = label
			predict.Groups = cc
			predict.Sample = ss
			predict.Key = k
			predict.Count = int(c.Status.Count)
			prediction[k] = predict
			// TODO : Do we really need the Samples ?
			s := fmt.Sprintf("%d -> %d", cfg.LookBack, cfg.LookAhead)
			if _, ok := c.Status.Samples[s]; !ok {
				c.Status.Samples[s] = make(map[Sequence]Sample)
			}
			c.Status.Samples[s][k] = Sample{
				Key:    k,
				Events: cc,
			}
		}
	}

	// add the new OpenValue at the end
	c.sequence.Value = s
	c.sequence = c.sequence.Next()

	return prediction, *c.Status
}

func (c *HMM) addKey(cfg HMMConfig, values []string, s string) (Sequence, Predictions, int, int) {
	// gather all available values
	vv := append(values, s)
	// we want to extract the value from the given values + the new one
	k := vv[len(vv)-(cfg.LookBack+cfg.LookAhead) : len(vv)-cfg.LookAhead]
	key := NewSequence(k)

	v := vv[len(vv)-cfg.LookAhead:]
	value := NewSequence(v)
	// if we have not encountered that meany values yet ... just skip
	if strings.Contains(string(key), "<nil>") {
		return "", Predictions{Values: []*Prediction{}}, 0, 0
	}

	// init the counter map for this key, if it s not there
	if _, ok := c.hmm[key]; !ok {
		c.hmm[key] = make(map[Sequence]int)
	}
	if _, ok := c.hmm[key][value]; !ok {
		c.hmm[key][value] = 0
	}

	// work to make the prediction by shifting the key for the desired target size
	kk := vv[len(vv)-cfg.LookBack:]
	pKey := NewSequence(kk)
	var predictions Predictions
	var sample int
	if !strings.Contains(string(pKey), "<nil>") {
		predictions, sample = c.predict(pKey)
	}

	// add the value to the counter map, note we do this after we make the prediction
	// to avoid affecting it by itself
	c.hmm[key][value]++

	// we also return how many samples we have for the given key
	// return also the number of other options for this sequence
	return pKey, predictions, sample, len(c.hmm[key])
}

func (c *HMM) predict(key Sequence) (Predictions, int) {
	predictions := Predictions{
		Values: make([]*Prediction, 0),
	}
	var s int
	if count, ok := c.hmm[key]; ok {
		// s is the number of events

		// TODO : make sure we find a better way to preserve the order in executions
		for v, cc := range count {
			predictions.Values = append(predictions.Values, NewPrediction(v, cc))
			s += cc
		}
		sort.Sort(sort.Reverse(predictions.Values))
		for _, pred := range predictions.Values {
			pred.Probability = float64(pred.Occur) / float64(s)
		}
	}
	return predictions, s
}
