package position

import (
	"math"
	"math/rand"
	"testing"
	"time"

	exchangelocal "github.com/drakos74/free-coin/client/local"
	"github.com/drakos74/free-coin/internal/model"
	userlocal "github.com/drakos74/free-coin/user/local"
)

func TestPosition_Track(t *testing.T) {

	u, err := userlocal.NewUser("")
	if err != nil {
		t.Fail()
	}

	e := exchangelocal.NewMockExchange()
	priceIncrease(e)

	tracker := newTracker("", u, e)

	for i := 0; i < 150; i++ {
		tracker.track()
	}

}

func priceIncrease(e *exchangelocal.MockExchange) {
	for i := 0; i < 150; i++ {
		div := 5 * rand.Float64()
		if rand.Float64() > 0.5 {
			div = -1 * div
		}
		price := float64(10 + i)
		if i > 100 {
			price -= float64(10 * (i - 100))
		}
		p := model.Position{
			MetaData: model.MetaData{
				OpenTime:    time.Now(),
				CurrentTime: time.Now().Add(time.Duration(i) * time.Minute),
			},
			Coin:         "x-test",
			Type:         model.Buy,
			OpenPrice:    10,
			CurrentPrice: math.Round(price + div),
			Volume:       1,
		}
		e.AddOpenPositionResponse(model.FullBatch(p))
	}
}
