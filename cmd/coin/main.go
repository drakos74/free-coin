package main

import (
	"context"
	"time"

	"github.com/drakos74/free-coin/client/kraken"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/algo/model"
	"github.com/drakos74/free-coin/internal/algo/processor"
	cointime "github.com/drakos74/free-coin/time"
	"github.com/drakos74/free-coin/user/telegram"
	"github.com/rs/zerolog/log"
)

func main() {

	ctx, cnl := context.WithCancel(context.Background())

	// TODO : make the implementation

	client := kraken.New(ctx, cointime.ThisInstant(), 10*time.Second)

	// TODO : make the implementation
	user, err := telegram.NewBot()
	if err != nil {
		if err != nil {
			panic(err.Error())
		}
	}
	err = user.Run(ctx)
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
