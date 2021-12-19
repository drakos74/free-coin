package xmath

import (
	"fmt"
	"strings"
)

type N interface {
	Op(op Op) N
	Dop(dop Dop) N
}

// CartesianProduct finds all possible combinations of the given data matrix.
// follows the same logic as https://stackoverflow.com/questions/53244303/all-combinations-in-array-of-arrays
func CartesianProduct(data [][]float64, current int, length int) [][]float64 {
	result := make([][]float64, 0)
	if current == length {
		return result
	}

	subCombinations := CartesianProduct(data, current+1, length)
	size := len(subCombinations)

	for i := 0; i < len(data[current]); i++ {
		if size > 0 {
			for j := 0; j < size; j++ {
				combinations := make([]float64, 0)
				combinations = append(combinations, data[current][i])
				combinations = append(combinations, subCombinations[j]...)
				result = append(result, combinations)
			}
		} else {
			combinations := make([]float64, 0)
			combinations = append(combinations, data[current][i])
			result = append(result, combinations)
		}
	}

	return result
}

// TODO : clarify which methods mutate the vector and which not

type Cube []Matrix

func Cb(d int) Cube {
	cube := make([]Matrix, d)
	return cube
}

func (c Cube) String() string {
	builder := strings.Builder{}
	builder.WriteString("\n")
	for i := 0; i < len(c); i++ {
		builder.WriteString(fmt.Sprintf("[%d]", i))
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("%v", c[i].String()))
	}
	return builder.String()
}
