package math

import (
	"fmt"
	"testing"
)

func TestFFT(t *testing.T) {

	//FFT(Sine(1, 100, 1))

	spectrum := FFT(Series(1, 100))

	fmt.Printf("spectrum = %+v\n", spectrum)

	//FFT([]float64{1, 2, 3})

}
