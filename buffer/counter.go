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

// Prediction defines a prediction result with the computed probability
type Prediction struct {
	value       string
	probability float64
	options     int
	sample      int
}

// Add adds a string to the counter sequence.
func (c *Counter) Add(s string) map[string]Prediction {

	values := make([]string, 0)

	c.sequence.Do(func(i interface{}) {
		values = append(values, fmt.Sprintf("%v", i))
	})

	println(fmt.Sprintf("values = %+v", values))

	prediction := make(map[string]Prediction)

	for _, l := range c.config {
		println(fmt.Sprintf("s = %+v", s))
		if k, predict := c.addKey(l, values, s); predict != nil {
			prediction[k] = *predict
		}
		println(fmt.Sprintf("c.counter = %+v", c.counter))
	}

	// add the new value at the end
	c.sequence.Value = s
	c.sequence = c.sequence.Next()

	return prediction
}

func (c *Counter) addKey(l int, values []string, v string) (string, *Prediction) {
	println(fmt.Sprintf("l = %+v", l))
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

	println(fmt.Sprintf("k = %+v", key))
	println(fmt.Sprintf("v = %+v", v))

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
					value:       r,
					probability: float64(m) / float64(s),
					options:     len(count),
					sample:      s,
				}
			}
		}
	}
	return nil
}
