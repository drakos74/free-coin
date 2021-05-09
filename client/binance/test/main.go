package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/drakos74/free-coin/internal/account"

	"github.com/drakos74/free-coin/client/binance"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
}

func main() {

	//testOrderOrder()

	testAccountBalance()

}

func testOrderOrder() { //nolint

	exchange := binance.NewExchange(account.Drakos)

	order := model.TrackedOrder{
		Order: model.NewOrder(model.Coin("EOSUSDT")).
			WithType(model.Buy).
			Market().
			WithVolume(4.928900608719226).
			Create(),
		Key:   model.Key{},
		Time:  time.Time{},
		TxIDs: nil,
	}

	o, txids, err := exchange.OpenOrder(order)
	if err != nil {
		fmt.Printf("\nerr = %+v", err)
	}
	fmt.Printf("\ntxids = %+v", txids)
	fmt.Printf("\no = %+v", o)
}

func testAccountBalance() {

	exchange := binance.NewMarginExchange(account.Drakos)

	bb, err := exchange.Balance(context.Background(), nil)

	if err != nil {
		log.Fatalf("could not get balance: %v", err)
	}

	for c, b := range bb {
		fmt.Printf("\n%s b = %+v", c, b)
	}

}
