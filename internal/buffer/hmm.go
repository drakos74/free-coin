package buffer

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	cointime "github.com/drakos74/free-coin/internal/time"
)

const Delimiter = "|"

// HMMConfig defines the configuration for the hidden markov model analysis.
type HMMConfig struct {
	IgnoreValues []string `json:"ignore"`
	LookBack     int      `json:"lookback"`
	LookAhead    int      `json:"lookahead"`
}

func NewHMMConfig(back, ahead int, ignoreValues ...string) HMMConfig {
	return HMMConfig{
		IgnoreValues: ignoreValues,
		LookBack:     back,
		LookAhead:    ahead,
	}
}

type State struct {
	Count int     `json:"count"`
	EMP   float64 `json:"emp"`
}

func (s State) sum() float64 {
	return float64(s.Count)
}

// NewMultiHMM creates a new State.
func NewMultiHMM(config ...HMMConfig) *HMM {
	var m int
	for _, s := range config {
		if s.LookBack+s.LookAhead > m {
			m = s.LookBack + s.LookAhead
		}
	}
	return &HMM{
		max:    m,
		buffer: NewBuffer(m),
		Config: config,
		State:  make(map[Sequence]map[Sequence]State),
		Status: newStatus(),
	}
}

// HMMFromState creates a new model from a previous one.
func HMMFromState(hmm HMM) *HMM {
	var max int
	for _, s := range hmm.Config {
		if s.LookBack+s.LookAhead > max {
			max = s.LookBack + s.LookAhead
		}
	}
	return &HMM{
		max:    max,
		buffer: NewBuffer(max + 1),
		Config: hmm.Config,
		State:  hmm.State,
		Status: hmm.Status,
	}
}

// HMM counts occurrences in a sequence of strings.
// It implements effectively several hidden markov model of the n-grams lengths provided in the Config.
type HMM struct {
	max    int                             `json:"max"`
	buffer *Buffer                         `json:"buffer"`
	Config []HMMConfig                     `json:"config"`
	State  map[Sequence]map[Sequence]State `json:"state"`
	Status Status                          `json:"status"`
}

func (hmm *HMM) Save(filename string) error {
	dd, err := json.Marshal(hmm)
	if err != nil {
		return fmt.Errorf("cannot encode gru: %w", err)
	}
	return os.WriteFile(filename, dd, 0644)
}

func (hmm *HMM) Load(filename string) error {
	dd, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("cannot decode gru: %w", err)
	}
	return json.Unmarshal(dd, hmm)
}

// Prediction defines a prediction result with the computed Probability
type Prediction struct {
	// ID is a unique numeric id related to the prediction
	ID int64
	// Value for the prediction . Essentially the concatenated string of the predicted sequence
	Value Sequence
	// Occur is the number of occurrences for the bucket combination.
	state State
	// Probability for the bucket prediction
	Probability float64
	// EMP is the exponential moving probability e.g. on-the-fly calculated probability with integrated exponential decay
	EMP float64
}

func (p *Prediction) String() string {
	return fmt.Sprintf("prob = %v , values = %+v", p.Probability, p.Value)
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
	// Sample is the number of previous incidents of the source sequence that generated the bucket probability matrix
	Sample int
	// Groups is the number of groups / combinations of source sequences encountered of the given length.
	// TODO :assess the statistical significance of this
	Groups int
	// Count is the Count of invocations for this model
	Count int
	// Label is a string acting as metadata for the prediction
	Label string
}

// NewPrediction
// TODO : maybe better to choose a uuid, for now the unix second should be enough
func NewPrediction(s Sequence, st State) *Prediction {
	return &Prediction{
		ID:    cointime.ToNano(time.Now()),
		Value: s,
		state: st,
	}
}

// PredictionList is a sortable list of predictions
type PredictionList []*Prediction

// for sorting predictions

func (p PredictionList) Len() int           { return len(p) }
func (p PredictionList) Less(i, j int) bool { return p[i].state.sum() < p[j].state.sum() }
func (p PredictionList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Status reflects the bucket Status of the HMM it is the inverse map of the state
type Status struct {
	Count   int                           `json:"count"`
	Samples map[Sequence]map[Sequence]int `json:"sample"`
}

func newStatus() Status {
	return Status{
		Samples: make(map[Sequence]map[Sequence]int),
	}
}

// Add adds a string to the State sequence.
// TODO : reverse the logic by accepting the result instead of the input.
// (This should allow us to filter out irrelevant data and save space,
// Note : State is expensive in terms of memory storage )
func (hmm *HMM) Add(s string) Status {
	// We cant allow our delimiter char to be used within the values.
	if strings.Contains(s, Delimiter) {
		panic(any(fmt.Sprintf("illegal character found '%s' in '%s'", Delimiter, s)))
	}
	hmm.Status.Count++
	if _, ok := hmm.buffer.Push(s); ok {
		for _, cfg := range hmm.Config {
			hmm.digest(cfg, hmm.buffer.GetAsStrings(false))
		}
	}
	return hmm.Status
}

//func (c *HMM) predict(){
//	predict.Label = label
//	predict.Groups = cc
//	predict.Sample = ss
//	predict.Key = k
//	predict.Count = int(c.Status.Count)
//	prediction[k] = predict
//	// TODO : Do we really need the Samples ?
//	s := fmt.Sprintf("%d -> %d", cfg.LookBack, cfg.LookAhead)
//	if _, ok := c.Status.Samples[s]; !ok {
//		c.Status.Samples[s] = make(map[Sequence]Sample)
//	}
//	c.Status.Samples[s][k] = Sample{
//		Key:    k,
//		Events: cc,
//	}
//}

func (hmm *HMM) digest(cfg HMMConfig, vv []string) int {
	// we want to extract the value from the given values + the new one
	k := vv[len(vv)-(cfg.LookAhead+cfg.LookBack) : len(vv)-cfg.LookAhead]
	key := NewSequence(k)
	v := vv[len(vv)-cfg.LookAhead:]
	value := NewSequence(v)

	// if we have not encountered that many values yet ... just skip
	if strings.Contains(string(key), "<nil>") {
		return 0
	}

	// init the counter map for this key, if it s not there
	if _, ok := hmm.State[key]; !ok {
		hmm.State[key] = make(map[Sequence]State)
	}
	if _, ok := hmm.State[key][value]; !ok {
		hmm.State[key][value] = State{}
	}

	// add the value to the counter map, note we do this after we make the prediction
	// to avoid affecting it by itself
	// do exponential adjustment
	count := 0
	for _, st := range hmm.State[key] {
		count += st.Count
	}
	// increment the counter
	st := hmm.State[key][value]
	st.Count++
	// let's put some more weight on the recent results.
	// TODO : but do this in a more sophisticated way
	st.EMP += 2 * float64(count)
	hmm.State[key][value] = st
	// ignore the irrelevant values to save space in the status history
	for _, ignoredValue := range cfg.IgnoreValues {
		if ignoredValue == string(value) {
			return len(hmm.State[key])
		}
	}
	if _, ok := hmm.Status.Samples[value]; !ok {
		hmm.Status.Samples[value] = make(map[Sequence]int)
	}
	if _, ok := hmm.Status.Samples[value][key]; !ok {
		hmm.Status.Samples[value][key] = 0
	}
	hmm.Status.Samples[value][key] = hmm.Status.Samples[value][key] + 1
	// we also return how many samples we have for the given key
	// return also the number of other options for this sequence
	return len(hmm.State[key])
}

func (hmm *HMM) Predict(key Sequence) map[Sequence]Predictions {
	predictions := make(map[Sequence]Predictions)
	// strip down key to our length
	for _, cfg := range hmm.Config {
		vv := key.Values()
		kk := NewSequence(vv[len(vv)-cfg.LookBack:])
		prediction := Predictions{
			Key:    kk,
			Values: make([]*Prediction, 0),
		}
		var s int
		if count, ok := hmm.State[kk]; ok {
			// s is the number of events
			// TODO : make sure we find a better way to preserve the order in executions
			for v, cc := range count {
				prediction.Values = append(prediction.Values, NewPrediction(v, cc))
				s += cc.Count
			}
			sort.Sort(sort.Reverse(prediction.Values))
			for _, pred := range prediction.Values {
				pred.Probability = float64(pred.state.Count) / float64(s)
				pred.EMP = pred.state.EMP / math.Pow(float64(s), 2)
			}
		}
		prediction.Sample = s
		predictions[kk] = prediction
	}

	return predictions
}
