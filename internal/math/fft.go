package math

import (
	"math/cmplx"
	"sort"

	"github.com/mjibson/go-dsp/fft"
)

func FFT(xx []float64) *Spectrum {
	cc := fft.FFTReal(xx)

	ss := newSpectrum()
	for i, n := range cc {
		if i > len(cc)/2 {
			continue
		}
		r := cmplx.Abs(n)
		ss.add(RNum{
			Amplitude: r,
			Frequency: i,
			Cos:       cmplx.Cos(n),
		})
	}

	sort.Sort(sort.Reverse(spectrums(ss.Values)))

	return ss
}

// Spectrum is a collection of spectra
type Spectrum struct {
	Values    []RNum
	Amplitude float64
}

func newSpectrum() *Spectrum {
	return &Spectrum{
		Values: make([]RNum, 0),
	}
}

func (s *Spectrum) add(r RNum) {
	s.Values = append(s.Values, r)
	s.Amplitude += r.Amplitude
}

func (s *Spectrum) Mean() float64 {
	return s.Amplitude / float64(len(s.Values))
}

// RNum defines a complex number attrinutes
type RNum struct {
	Amplitude float64
	Frequency int
	Cos       complex128
}

type coefficients []complex128

func (c coefficients) Len() int           { return len(c) }
func (c coefficients) Less(i, j int) bool { return cmplx.Abs(c[i]) > cmplx.Abs(c[j]) }
func (c coefficients) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }

type spectrums []RNum

func (s spectrums) Len() int           { return len(s) }
func (s spectrums) Less(i, j int) bool { return s[i].Amplitude < s[j].Amplitude }
func (s spectrums) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
