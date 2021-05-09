package main

import (
	"context"
	"fmt"

	"github.com/drakos74/free-coin/internal/account"

	"github.com/drakos74/free-coin/client/kraken"
)

func main() {

	client := kraken.NewExchange(account.Drakos)

	pp, err := client.CurrentPrice(context.Background())

	fmt.Printf("\nerr = %+v", err)
	fmt.Printf("\npp = %+v", pp)
}
