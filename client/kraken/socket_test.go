package kraken

import (
	"fmt"
	"testing"

	"github.com/drakos74/free-coin/internal/account"
	"github.com/drakos74/free-coin/internal/model"
)

func TestRun(t *testing.T) {

	exchange := NewExchange(account.Drakos)
	fmt.Printf("exchange = %+v\n", exchange)

	socket := NewSocket(model.BTC)
	ch, err := socket.Run()
	if err != nil {
		t.Fail()
	}

	for signal := range ch {
		fmt.Printf("book = %+v\n", signal.Book)
		fmt.Printf("tick = %+v\n", signal.Tick)
	}

}
