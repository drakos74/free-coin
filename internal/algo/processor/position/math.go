package position

import (
	"fmt"
	"math"

	coinmath "github.com/drakos74/free-coin/internal/math"
)

func interpolate(points [][]float64) (float64, error) {

	l := len(points)
	fmt.Printf("l = %+v\n", l)
	x0 := make([]float64, l-1)
	x := make([]float64, l)
	y0 := make([]float64, l-1)
	y := make([]float64, l)

	s := 0.0
	for i, p := range points {
		if i == 0 {
			s = p[1]
		}
		xx := (p[1] - s) / 1000000
		yy := p[0]
		x[i] = xx
		y[i] = yy
		if i < l-1 {
			x0[i] = xx
			y0[i] = yy
		}
	}

	l0, err := coinmath.Fit(x0, y0, 2)
	if err != nil {
		return 0, fmt.Errorf("%w", err)
	}
	fmt.Printf("y = %+v\n", y)
	l1, err := coinmath.Fit(x, y, 2)
	if err != nil {
		return 0, fmt.Errorf("%w", err)
	}

	fmt.Printf("l1 = %+v\n", l1)

	ll := l1[2] - l0[2]

	if math.Abs(ll) >= 1.0 {
		return ll, nil
	}

	return 0, fmt.Errorf("not enough data to interpolate")
}
