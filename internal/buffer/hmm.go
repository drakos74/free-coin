package buffer

import (
	"container/ring"
	"fmt"
	"sort"
	"strings"
)

// HMMConfig defines the configuration for the hidden markov model analysis.
type HMMConfig struct {
	PrevSize   int
	TargetSize int
}

// NewMultiHMM creates a new hmm.
func NewMultiHMM(config ...HMMConfig) *HMM {
	var max int
	for _, s := range config {
		if s.PrevSize+s.TargetSize > max {
			max = s.PrevSize + s.TargetSize
		}
	}
	return &HMM{
		max:      max,
		sequence: ring.New(max + 1),
		config:   config,
		hmm:      make(map[string]map[string]int),
	}
}

// HMM counts occurrences in a sequence of strings.
// It implements effectively several hidden markov model of the n-grams lengths provided in the config.
type HMM struct {
	max      int
	sequence *ring.Ring
	config   []HMMConfig
	hmm      map[string]map[string]int
}

// Prediction defines a prediction result with the computed Probability
type Prediction struct {
	Value       string
	Probability float64
	Options     int
	Sample      int
}

// Add adds a string to the hmm sequence.
// TODO : reverse the logic by accepting the result instead of the input.
// (This should allow us to filter out irrelevant data adn save space,
// Note : hmm is expensive in terms of memory storage )
func (c *HMM) Add(s string) map[string]Prediction {

	values := make([]string, 0)

	c.sequence.Do(func(i interface{}) {
		values = append(values, fmt.Sprintf("%v", i))
	})

	prediction := make(map[string]Prediction)

	for _, cfg := range c.config {
		if k, predict := c.addKey(cfg, values, s); predict != nil {
			prediction[k] = *predict
		}
	}

	// add the new Value at the end
	c.sequence.Value = s
	c.sequence = c.sequence.Next()

	return prediction
}

func (c *HMM) addKey(cfg HMMConfig, values []string, s string) (string, *Prediction) {
	// we want to extract the value from the given values + the new one
	valueSize := cfg.TargetSize - 1
	// create the key for each of the configs
	keySize := cfg.PrevSize + valueSize
	k := values[len(values)-keySize:]
	key := strings.Join(k, ":")

	v := append(values[len(values)-valueSize:], s)
	value := strings.Join(k, ":")
	// if we have not encountered that meany values yet ... just skip
	if strings.Contains(key, "<nil>") {
		return "", nil
	}

	// init the counter map for this key, if it s not there
	if _, ok := c.hmm[key]; !ok {
		c.hmm[key] = make(map[string]int)
	}
	if _, ok := c.hmm[key][value]; !ok {
		c.hmm[key][value] = 0
	}

	// work to make the prediction by shifting the key for the desired target size
	kk := append(k[cfg.TargetSize:], v...)
	pKey := strings.Join(kk, ":")
	var prediction *Prediction
	if !strings.Contains(pKey, "<nil>") {
		prediction = c.predict(pKey)
	}

	// add the value to the counter map, note we do this after we make the prediction
	// to avoid affecting it by itself
	c.hmm[key][value]++

	return pKey, prediction
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
