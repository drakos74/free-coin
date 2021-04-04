package main

import (
	"context"
	"fmt"

	"github.com/drakos74/free-coin/client/kraken"
)

func main() {

	client := kraken.NewExchange(context.Background())

	pp, err := client.CurrentPrice(context.Background())

	fmt.Println(fmt.Sprintf("err = %+v", err))
	fmt.Println(fmt.Sprintf("pp = %+v", pp))
}
