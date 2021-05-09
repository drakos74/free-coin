package main

import (
	"fmt"
	"log"

	"github.com/drakos74/free-coin/internal/account"

	"github.com/drakos74/free-coin/client/binance"
	"github.com/drakos74/free-coin/client/binance/model"
)

func main() {

	exc := binance.NewExchange(account.Drakos)

	info := exc.CoinInfo("ADAUSDT")

	lotSize, err := model.ParseLOTSize(info.Filters)

	if err != nil {
		log.Fatalf("could not parse lot size: %v", err)
	}

	fmt.Printf("\nl = %+v", lotSize)

	f := 16.7
	v := lotSize.Adjust(f)

	if f != v {
		fmt.Printf("\nf = %+v", f)
		fmt.Printf("/nv = %+v", v)
		log.Fatalf("value should be the same")
	}

}
