package stats

import (
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatIntToString(t *testing.T) {
	for i := -10; i <= 10; i++ {
		s := strconv.FormatInt(int64(i), 10)
		sign := ""
		if i < 0 {
			sign = "-"
		}
		println(fmt.Sprintf("s = %+v", s))
		assert.Equal(t, fmt.Sprintf("%s%d", sign, int(math.Abs(float64(i)))), s)
	}
}
