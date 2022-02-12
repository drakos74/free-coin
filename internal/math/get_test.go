package math

import (
	"fmt"
	"testing"
)

func TestGenerateFloats(t *testing.T) {

	ss := GenerateFloats(100, VaryingSine(50, 10, 0.1))

	for _, s := range ss {
		fmt.Printf("s = %+v\n", s)
	}

}
