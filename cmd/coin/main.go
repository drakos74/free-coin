package main

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/drakos74/free-coin/internal/algo/processor"

	"github.com/drakos74/free-coin/internal/algo/model"
	"github.com/drakos74/free-coin/internal/api"

	coin "github.com/drakos74/free-coin/internal"
)

func main() {

	ctx, _ := context.WithCancel(context.Background())

	// TODO : make the implementation
	var client api.TradeClient

	// TODO : make the implementation
	var user api.UserInterface
	err := user.Run(ctx)
	if err != nil {
		panic(err.Error())
	}

	overWatch := coin.New(client, user)
	go overWatch.Run()

	// TODO : define coins
	var coins map[model.Coin]*coin.Engine

	for c, _ := range coins {
		err := overWatch.Start(c, coin.Void(), processor.MultiStats(client, user))
		if err != nil {
			log.Error().Str("coin", string(c)).Err(err).Msg("could not start engine")
		}
	}

	// this is a long running task ... lets keep the main thread occupied
	<-ctx.Done()

}
