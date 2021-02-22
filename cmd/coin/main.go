package main

import (
	"context"
	"time"

	"github.com/drakos74/free-coin/client/local"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/free-coin/internal/storage/file/json"
	"github.com/drakos74/free-coin/user"

	"github.com/drakos74/free-coin/client/kraken"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/model"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {

	ctx, cnl := context.WithCancel(context.Background())
	exchange := kraken.NewExchange(ctx)
	upstream := func(since int64) (api.Client, error) {
		return kraken.NewClient(since, 15*time.Second), nil
	}
	persistence := func(shard string) (storage.Persistence, error) {
		return json.NewJsonBlob("trades", shard), nil
	}
	client := local.NewClient(cointime.LastXHours(96)).
		WithUpstream(upstream).
		WithPersistence(persistence)
	//client := kraken.NewClient(ctx, cointime.LastXHours(99), 10*time.Second)

	//user, err := telegram.NewBot()
	user, err := user.NewVoid()
	if err != nil {
		if err != nil {
			panic(err.Error())
		}
	}

	err = user.Run(context.TODO())
	if err != nil {
		panic(err.Error())
	}

	overWatch := coin.New(client, user)
	go overWatch.Run(ctx)

	singals := make(chan api.Signal)

	statsProcessor := processor.MultiStats(exchange, user, singals)
	//positionProcessor := processor.Position(exchange, user)
	tradeProcessor := processor.Trade(exchange, user, singals)
	for _, c := range model.Coins {
		err := overWatch.Start(c, coin.Log(),
			statsProcessor,
			//positionProcessor,
			tradeProcessor,
		)
		if err != nil {
			log.Error().Str("coin", string(c)).Err(err).Msg("could not start engine")
		}
	}

	// this is a long running task ... lets keep the main thread occupied
	<-ctx.Done()
	cnl()
}
