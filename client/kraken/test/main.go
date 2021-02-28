package main

import (
	"context"
	"fmt"
	"time"

	"github.com/drakos74/free-coin/client/kraken"
)

func main() {

	history, err := kraken.NewHistory(context.Background())
	if err != nil {
		panic(err.Error())
	}
	trades := history.Get(time.Now().Add(-1000 * time.Hour))
	for _, trade := range trades {
		fmt.Println(fmt.Sprintf("trade = %+v", trade))
	}

}
