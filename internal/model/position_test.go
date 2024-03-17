package model

import (
	"fmt"
	"math"
	"testing"
	"time"

	math2 "github.com/drakos74/free-coin/internal/math"

	"github.com/stretchr/testify/assert"
)

func TestPositionTracking(t *testing.T) {

	type expectation struct {
		pnl        float64
		price      float64
		iterations int
	}

	type test struct {
		exp       expectation
		t         Type
		transform func(i int) float64
	}

	tests := map[string]test{
		"buy-loss": {
			t: Buy,
			transform: func(i int) float64 {
				return -1 * float64(i)
			},
			exp: expectation{
				pnl:        -0.055,
				price:      950,
				iterations: 50,
			},
		},
		"buy-profit": {
			t: Buy,
			transform: func(i int) float64 {
				// note sin will first go up and then down ...
				return 100 * math.Sin(float64(i)/100)
			},
			exp: expectation{
				pnl:        0.055,
				price:      1050,
				iterations: 60,
			},
		},
		"buy-profit-long": {
			t: Buy,
			transform: func(i int) float64 {
				// note sin will first go up and then down ...
				return 100 + 100*math.Cos(math.Pi+float64(i)/100)
			},
			exp: expectation{
				pnl:        0.12,
				price:      1120,
				iterations: 175,
			},
		},
		"sell-loss": {
			t: Sell,
			transform: func(i int) float64 {
				return float64(i)
			},
			exp: expectation{
				pnl:        -0.05,
				price:      1050,
				iterations: 50,
			},
		},
		"sell-profit": {
			t: Sell,
			transform: func(i int) float64 {
				// note cos will first go down and then up ...
				return -100 * math.Sin(float64(i)/100)
			},
			exp: expectation{
				pnl:        0.05,
				price:      950,
				iterations: 60,
			},
		},
		"sell-profit-long": {
			t: Sell,
			transform: func(i int) float64 {
				// note sin will first go up and then down ...
				return -100 - 100*math.Cos(math.Pi+float64(i)/100)
			},
			exp: expectation{
				pnl:        0.12,
				price:      875,
				iterations: 175,
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {

			now := time.Now()

			order := NewOrder("TEST").
				WithType(tt.t).
				Market().
				WithVolume(0.1).
				WithPrice(1000).
				CreateTracked(Key{}, time.Now(), "")

			// note : we should have similar results if we change the samples to 5 :)
			cfg := Track(1*time.Second, 3)

			pos := OpenPosition(order, []*TrackingConfig{cfg})

			for i := 0; i < 1000; i++ {
				now = now.Add(100 * time.Millisecond)
				p := pos.Update(true, NewTick(1000+tt.transform(i), 1, 1, now), []*TrackingConfig{cfg})
				pp, pnl, _, _ := AssessTrend(map[Key]Position{
					Key{
						Coin:     "TEST",
						Duration: 1 * time.Second,
					}: p,
				}, 0.05, 0.05)
				if len(pp) > 0 {
					// we can retrieve an action from the assessment
					assert.True(t, pnl[0] <= (tt.exp.pnl+0.01) && pnl[0] >= (tt.exp.pnl-0.01), fmt.Sprintf("expected pnl shold be around the given value [%v] but was %v", tt.exp.pnl, pnl[0]))
					assert.True(t, p.CurrentPrice <= (tt.exp.price+15) && p.CurrentPrice >= (tt.exp.price-15), fmt.Sprintf("expected pnl shold be around the given value [%v] but was %v", tt.exp.price, p.CurrentPrice))
					assert.True(t, i <= (tt.exp.iterations+5) && i >= (tt.exp.iterations-5), fmt.Sprintf("expected pnl shold be around the given value [%v] but was %v", tt.exp.iterations, i))
					return
				}
			}
			// if we went through everything without an action thats not good :(
			assert.Fail(t, "should have acted before")
		})
	}

}

func TestPositionCloseConditionI(t *testing.T) {

	x := []float64{0.00, 0.50, 1.00, 1.50, 2.00, 2.50}
	y := []float64{0.62, 0.70, 0.72, 0.70, 0.59, 0.67}

	a, _ := math2.Fit(x, y, 2)
	fmt.Printf("a = %+v\n", a)
}
