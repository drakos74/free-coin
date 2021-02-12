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
