package main

import (
	"context"
	"time"

	"github.com/drakos74/free-coin/client/kraken"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/algo/model"
	"github.com/drakos74/free-coin/internal/algo/processor"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/drakos74/free-coin/user/telegram"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {

	ctx, cnl := context.WithCancel(context.Background())

	//client := kraken.New(ctx, cointime.LastXHours(48), 10*time.Second)
	client := kraken.New(ctx, cointime.ThisInstant(), 10*time.Second)

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

	statsProcessor := processor.MultiStats(client, user)
	positionProcessor := processor.Position(client, user)
	for _, c := range model.Coins {
		err := overWatch.Start(c, coin.Void(),
			statsProcessor,
			positionProcessor,
		)
		if err != nil {
			log.Error().Str("coin", string(c)).Err(err).Msg("could not start engine")
		}
	}

	// this is a long running task ... lets keep the main thread occupied
	<-ctx.Done()
	cnl()
}
