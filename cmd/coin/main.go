package main

import (
	"context"

	"github.com/drakos74/free-coin/coinapi"

	"github.com/rs/zerolog/log"

	"github.com/drakos74/free-coin/internal/algo/processor"

	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/algo/model"
)

func main() {

	ctx, cnl := context.WithCancel(context.Background())

	// TODO : make the implementation
	var client coinapi.TradeClient

	// TODO : make the implementation
	var user coinapi.UserInterface
	err := user.Run(ctx)
	if err != nil {
		panic(err.Error())
	}

	overWatch := coin.New(client, user)
	go overWatch.Run()

	for _, c := range model.Coins {
		err := overWatch.Start(c, coin.Void(), processor.MultiStats(client, user))
		if err != nil {
			log.Error().Str("coin", string(c)).Err(err).Msg("could not start engine")
		}
	}

	// this is a long running task ... lets keep the main thread occupied
	<-ctx.Done()
	cnl()
}
