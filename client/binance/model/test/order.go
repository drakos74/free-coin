package main

import (
	"fmt"
	"log"

	"github.com/drakos74/free-coin/client/binance"
	"github.com/drakos74/free-coin/client/binance/model"
)

func main() {

	exc := binance.NewExchange(binance.Free)

	info := exc.CoinInfo("ADAUSDT")

	lotSize, err := model.ParseLOTSize(info.Filters)

	if err != nil {
		log.Fatalf("could not parse lot size: %v", err)
	}

	fmt.Println(fmt.Sprintf("l = %+v", lotSize))

	f := 16.7
	v := lotSize.Adjust(f)

	if f != v {
		fmt.Println(fmt.Sprintf("f = %+v", f))
		fmt.Println(fmt.Sprintf("v = %+v", v))
		log.Fatalf("value should be the same")
	}

}
