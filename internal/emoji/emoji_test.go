package emoji

import (
	"fmt"
	"math"
	"testing"
)

func TestMapLog10(t *testing.T) {

	c := 0.00001
	for i := 0; i < 10; i++ {
		j := c * math.Pow10(i-5)
		if i < 5 {
			j = -1 * c * math.Pow10(i)
		}

		fmt.Printf("j = %+v\n", j)

		s := MapLog10(j)
		fmt.Printf("%v => s = %+v\n", j, s)
	}

}
