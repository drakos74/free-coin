package main

import (
	"context"
	"fmt"
	"log"
	"time"

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

func testOrderOrder() {
	exchange := binance.NewExchange(binance.Free)

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
		fmt.Println(fmt.Sprintf("err = %+v", err))
	}
	fmt.Println(fmt.Sprintf("txids = %+v", txids))
	fmt.Println(fmt.Sprintf("o = %+v", o))
}

func testAccountBalance() {

	exchange := binance.NewExchange(binance.Free)

	bb, err := exchange.Balance(context.Background())

	if err != nil {
		log.Fatalf("could not get balance: %v", err)
	}

	for c, b := range bb {
		fmt.Println(fmt.Sprintf("%s b = %+v", c, b))
	}

}
