package math

import "gonum.org/v1/gonum/mat"

// Fit fits the given series of x and y into a polynomial function of the given degree
// out put is a vector with the coefficients of the corresponding powers of x
// c[0] + c[1]x + c[2]x^2 + c[3]x^3 + ...
func Fit(x, y []float64, degree int) ([]float64, error) {

	a := vandermonde(x, degree)
	b := mat.NewDense(len(y), 1, y)
	c := mat.NewDense(degree+1, 1, nil)

	qr := new(mat.QR)
	qr.Factorize(a)

	err := qr.SolveTo(c, false, b)

	v := c.ColView(0)
	cc := make([]float64, v.Len())
	for i := 0; i < v.Len(); i++ {
		cc[i] = v.AtVec(i)
	}
	return cc, err
}

func vandermonde(a []float64, degree int) *mat.Dense {
	x := mat.NewDense(len(a), degree+1, nil)
	for i := range a {
		for j, p := 0, 1.; j <= degree; j, p = j+1, p*a[i] {
			x.Set(i, j, p)
		}
	}
	return x
}
