package buffer

import (
	"container/ring"
	"fmt"
	"sort"
	"strings"
)

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
		config:   config,
		hmm:      make(map[string]map[string]int),
		status:   newStatus(),
	}
}

// HMM counts occurrences in a sequence of strings.
// It implements effectively several hidden markov model of the n-grams lengths provided in the config.
type HMM struct {
	count    int64
	max      int
	sequence *ring.Ring
	config   []HMMConfig
	hmm      map[string]map[string]int
	status   *Status
}

// Prediction defines a prediction result with the computed Probability
type Prediction struct {
	// Value for the prediction . Essentially the concatenated string of the predicted sequence
	Value string
	// Probability for the current prediction
	Probability float64
	// Options is the pool of possible value combinations from which the current one was the winner
	Options int
	// Sample is the number of previous incidents of the source sequence that generated the current probability matrix
	Sample int
	// Count is the number of events processed by the model
	Count int
	// Groups is the number of groups / combinations of source sequences encountered.
	Groups int
	// Label is a string acting as metadata for the prediction
	Label string
}

type Sample struct {
	Key    string
	Events int
}

// Status reflects the current status of the HMM
type Status struct {
	Count   int64
	Samples map[string]map[string]Sample
}

func newStatus() *Status {
	return &Status{
		Samples: make(map[string]map[string]Sample),
	}
}

// Add adds a string to the hmm sequence.
// TODO : reverse the logic by accepting the result instead of the input.
// (This should allow us to filter out irrelevant data adn save space,
// Note : hmm is expensive in terms of memory storage )
func (c *HMM) Add(s string, label string) (map[string]Prediction, Status) {

	c.status.Count++

	values := make([]string, 0)

	c.sequence.Do(func(i interface{}) {
		values = append(values, fmt.Sprintf("%v", i))
	})

	prediction := make(map[string]Prediction)

	for _, cfg := range c.config {
		if k, predict, cc := c.addKey(cfg, values, s); predict != nil {
			predict.Label = label
			predict.Count = int(c.status.Count)
			predict.Groups = cc
			prediction[k] = *predict
			s := fmt.Sprintf("%d -> %d", cfg.LookBack, cfg.LookAhead)
			if _, ok := c.status.Samples[s]; !ok {
				c.status.Samples[s] = make(map[string]Sample)
			}
			c.status.Samples[s][k] = Sample{
				Key:    k,
				Events: cc,
			}
		}
	}

	// add the new Value at the end
	c.sequence.Value = s
	c.sequence = c.sequence.Next()

	return prediction, *c.status
}

func (c *HMM) addKey(cfg HMMConfig, values []string, s string) (string, *Prediction, int) {
	// gather all available values
	vv := append(values, s)
	// we want to extract the value from the given values + the new one
	k := vv[len(vv)-(cfg.LookBack+cfg.LookAhead) : len(vv)-cfg.LookAhead]
	key := strings.Join(k, ":")

	v := vv[len(vv)-cfg.LookAhead:]
	value := strings.Join(v, ":")
	// if we have not encountered that meany values yet ... just skip
	if strings.Contains(key, "<nil>") {
		return "", nil, 0
	}

	// init the counter map for this key, if it s not there
	if _, ok := c.hmm[key]; !ok {
		c.hmm[key] = make(map[string]int)
	}
	if _, ok := c.hmm[key][value]; !ok {
		c.hmm[key][value] = 0
	}

	// work to make the prediction by shifting the key for the desired target size
	kk := vv[len(vv)-cfg.LookBack:]
	pKey := strings.Join(kk, ":")
	var prediction *Prediction
	if !strings.Contains(pKey, "<nil>") {
		prediction = c.predict(pKey)
	}

	// add the value to the counter map, note we do this after we make the prediction
	// to avoid affecting it by itself
	c.hmm[key][value]++

	// we also return how many samples we have for the given key
	return pKey, prediction, len(c.hmm[key])
}

func (c *HMM) predict(key string) *Prediction {
	if count, ok := c.hmm[key]; ok {
		var m int
		var r string
		var s int

		// TODO : make sure we find a better way to preserve the order in executions
		keys := make([]string, len(count))
		for k := range count {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, kk := range keys {
			v := count[kk]
			s += v
			if v > m {
				r = kk
				m = v
			}
		}

		if s > 0 {
			return &Prediction{
				Value:       r,
				Probability: float64(m) / float64(s),
				Options:     len(count),
				Sample:      s,
			}
		}
	}
	return nil
}
