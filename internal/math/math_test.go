package math

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormat(t *testing.T) {

	type test struct {
		input  float64
		output string
	}

	tests := map[string]test{
		"0": {
			input:  0,
			output: "0.00",
		},
		"-1": {
			input:  -1,
			output: "-1.00",
		},
		"+1": {
			input:  1,
			output: "1.00",
		},
		"5": {
			input:  1.5555,
			output: "1.56",
		},
		"4": {
			input:  1.4444,
			output: "1.44",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := Format(tt.input)
			assert.Equal(t, tt.output, s)
		})
	}

}

func TestO10(t *testing.T) {

	type test struct {
		input  float64
		output int
	}

	tests := map[string]test{
		"0": {
			input:  0,
			output: -1*math.MaxInt64 - 1,
		},
		"-1": {
			input:  -1,
			output: 0,
		},
		"1": {
			input:  1,
			output: 0,
		},
		"-0.134": {
			input:  -0.134,
			output: 0,
		},
		"0.1654": {
			input:  0.1654,
			output: 0,
		},
		"-0.0734": {
			input:  -0.0734,
			output: 1,
		},
		"0.02654": {
			input:  0.02654,
			output: 1,
		},
		"0.0143242": {
			input:  0.0143242,
			output: 1,
		},
		"0.00167676": {
			input:  0.00167676,
			output: 2,
		},
		"-0.0156": {
			input:  -0.0156,
			output: 1,
		},
		"-0.001123": {
			input:  -0.001123,
			output: 2,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := O10(tt.input)
			assert.Equal(t, tt.output, s)
		})
	}

}

func TestO10_Sequence(t *testing.T) {

	limit := 10000000

	boundaries := map[float64]int64{
		-5000.000000: 3,
		-999.999000:  2,
		-99.999000:   1,
		-9.999000:    0,
		-0.099000:    1,
		-0.010000:    2,
		-0.001000:    3,
		0.000000:     -1*math.MaxInt64 - 1,
		0.001000:     3,
		0.002000:     2,
		0.011000:     1,
		0.100000:     0,
		10.000000:    1,
		100.000000:   2,
		1000.000000:  3,
	}

	var n int
	for i := 0; i < limit; i++ {
		f := float64(-1*limit/2+i) / 1000
		o := O10(f)
		if o != n {
			assert.Equal(t, boundaries[f], int64(o), fmt.Sprintf("f:o = %f -> %+v", f, o))
			n = o
		}
	}

}

func TestO2(t *testing.T) {

	type test struct {
		input  float64
		output int
	}

	tests := map[string]test{
		"0": {
			input:  0,
			output: -1*math.MaxInt64 - 1,
		},
		"-1": {
			input:  -1,
			output: 0,
		},
		"1": {
			input:  1,
			output: 0,
		},
		"-0.134": {
			input:  -0.134,
			output: 2,
		},
		"0.1654": {
			input:  0.1654,
			output: 1,
		},
		"-0.0734": {
			input:  -0.0734,
			output: 2,
		},
		"0.02654": {
			input:  0.02654,
			output: 3,
		},
		"0.0143242": {
			input:  0.0143242,
			output: 4,
		},
		"0.00167676": {
			input:  0.00167676,
			output: 6,
		},
		"-0.0156": {
			input:  -0.0156,
			output: 4,
		},
		"-0.001123": {
			input:  -0.001123,
			output: 6,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := O2(tt.input)
			assert.Equal(t, tt.output, s)
		})
	}

}

// Note : this is much more granular, but for our target of .05 -> 0.3 it only adds 2 levels
// (which actually might be enough to get a better insight)
func TestO2_Sequence(t *testing.T) {

	limit := 10000000

	boundaries := map[float64]int64{
		-5000.000000: 8,
		-2980.957000: 7,
		-1096.633000: 6,
		-403.428000:  5,
		-148.413000:  4,
		-54.598000:   3,
		-20.085000:   2,
		-7.389000:    1,
		-0.367000:    1,
		-0.135000:    2,
		-0.049000:    3,
		-0.018000:    4,
		-0.006000:    5,
		-0.002000:    6,
		0.000000:     -1*math.MaxInt64 - 1,
		0.00300:      5,
		0.007000:     4,
		0.001000:     6,
		0.019000:     3,
		0.050000:     2,
		0.136000:     1,
		2.719000:     1,
		7.390000:     2,
		20.086000:    3,
		54.599000:    4,
		148.414000:   5,
		403.429000:   6,
		1096.634000:  7,
		2980.958000:  8,
	}

	var n int
	for i := 0; i < limit; i++ {
		f := float64(-1*limit/2+i) / 1000
		o := O2(f)
		if o != n {
			assert.Equal(t, boundaries[f], int64(o), fmt.Sprintf("f:o = %f -> %+v", f, o))
			n = o
		}
	}

}
