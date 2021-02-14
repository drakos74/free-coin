package buffer

import (
	"container/ring"
	"fmt"
	"strings"
)

// NewCounter creates a new counter.
func NewCounter(sizes ...int) *Counter {
	var max int
	for _, s := range sizes {
		if s > max {
			max = s
		}
	}
	return &Counter{
		max:      max,
		sequence: ring.New(max + 1),
		config:   sizes,
		counter:  make(map[string]map[string]int),
	}
}

// Counter counts occurrences in a sequence of strings.
type Counter struct {
	max      int
	sequence *ring.Ring
	config   []int
	counter  map[string]map[string]int
}

// Prediction defines a prediction result with the computed Probability
type Prediction struct {
	Value       string
	Probability float64
	Options     int
	Sample      int
}

// Add adds a string to the counter sequence.
func (c *Counter) Add(s string) map[string]Prediction {

	values := make([]string, 0)

	c.sequence.Do(func(i interface{}) {
		values = append(values, fmt.Sprintf("%v", i))
	})

	prediction := make(map[string]Prediction)

	for _, l := range c.config {
		if k, predict := c.addKey(l, values, s); predict != nil {
			prediction[k] = *predict
		}
	}

	// add the new Value at the end
	c.sequence.Value = s
	c.sequence = c.sequence.Next()

	return prediction
}

func (c *Counter) addKey(l int, values []string, v string) (string, *Prediction) {
	// create the key for each of the configs
	k := values[len(values)-l:]
	key := strings.Join(k, ":")

	if strings.Contains(key, "<nil>") {
		return "", nil
	}

	if _, ok := c.counter[key]; !ok {
		c.counter[key] = make(map[string]int)
	}

	if _, ok := c.counter[key][v]; !ok {
		c.counter[key][v] = 0
	}

	kk := append(k[1:], v)
	pKey := strings.Join(kk, ":")
	var prediction *Prediction
	if !strings.Contains(pKey, "<nil>") {
		prediction = c.predict(pKey)
	}

	c.counter[key][v]++

	return pKey, prediction
}

func (c *Counter) predict(key string) *Prediction {
	if _, ok := c.counter[key]; ok {
		var m int
		var r string
		var s int
		if count, ok := c.counter[key]; ok {
			for k, c := range count {
				s += c
				if c > m {
					r = k
					m = c
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
	}
	return nil
}
