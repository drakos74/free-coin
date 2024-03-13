package net

import (
	"fmt"
	"testing"
)

func TestStrip(t *testing.T) {

	in := [][]float64{
		{1}, {2}, {3},
	}

	out := [][]float64{
		{1}, {2}, {3}, {4}, {5},
	}

	i, o := strip(out, in)

	fmt.Printf("i = %+v\n", i)
	fmt.Printf("o = %+v\n", o)

}
