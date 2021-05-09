package model

import (
	"fmt"
	"testing"
	"time"
)

func TestPositionTracking(t *testing.T) {

	type test struct {
		Type Type
	}

	tests := map[string]test{
		"buy-loss":    {Type: Buy},
		"sell-profit": {Type: Sell},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {

			now := time.Now()

			order := NewOrder("TEST").
				WithType(tt.Type).
				Market().
				WithVolume(0.1).
				WithPrice(100).
				CreateTracked(Key{}, time.Now())

			cfg := Track(1*time.Second, 10)

			pos := OpenPosition(order, cfg)

			for i := 0; i < 100; i++ {
				now = now.Add(100 * time.Millisecond)
				pos.Value(NewPrice(float64(i), now))
				fmt.Printf("\npos.Profit.Data = %+v", pos.Profit.Data)
			}

		})
	}

}
