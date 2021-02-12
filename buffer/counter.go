package buffer

import (
	"container/ring"
	"fmt"
	"strings"
)

type trackingKey []int

type trackedValue struct {
	emoji string
	value int
}

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

// Prediction defines a prediction result with the computed probability
type Prediction struct {
	value       string
	probability float64
	options     int
	sample      int
}

// Add adds a string to the counter sequence.
func (c *Counter) Add(s string) {

	values := make([]string, 0)

	c.sequence.Do(func(i interface{}) {
		values = append(values, fmt.Sprintf("%v", i))
	})

	prediction := make(map[string]Prediction)

	for _, l := range c.config {
		c.addKey(l, values, s)
		if k, predict := c.getKey(l, values, s); k != "" {
			prediction[k] = *predict
		}
	}

	// add the new value at the end
	c.sequence.Value = s
	c.sequence = c.sequence.Next()
}

func (c *Counter) getKey(l int, values []string, v string) (string, *Prediction) {
	// create the key for each of the configs
	k := values[l-1:]
	// ... replace the new value
	k = append(k[1:], v)
	key := strings.Join(k, ":")

	if strings.Contains(key, "<nil>") {
		return "", nil
	}

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
				return key, &Prediction{
					value:       r,
					probability: float64(m) / float64(s),
					options:     len(count),
					sample:      s,
				}
			}
		}
	}

	return "", nil
}

func (c *Counter) addKey(l int, values []string, v string) {
	// create the key for each of the configs
	k := values[l-1:]
	key := strings.Join(k, ":")

	if strings.Contains(key, "<nil>") {
		return
	}

	if _, ok := c.counter[key]; !ok {
		c.counter[key] = make(map[string]int)
	}

	if _, ok := c.counter[key][v]; !ok {
		c.counter[key][v] = 0
	} else {
		// capture the result of the previous prediction

	}

	c.counter[key][v]++
}
