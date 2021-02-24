package main

import (
	"context"
	"time"

	local2 "github.com/drakos74/free-coin/user/local"

	"github.com/drakos74/free-coin/client/kraken"

	"github.com/drakos74/free-coin/client/local"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/free-coin/internal/storage/file/json"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// The same as for coin/main.go
// just with a mock exchange client
// and local trades

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {

	ctx, cnl := context.WithCancel(context.Background())
	upstream := func(since int64) (api.Client, error) {
		return kraken.NewClient(since, 15*time.Second), nil
	}
	persistence := func(shard string) (storage.Persistence, error) {
		return json.NewJsonBlob("trades", shard), nil
	}
	// TODO : specify a concrete time
	client := local.NewClient(cointime.LastXHours(120)).
		WithUpstream(upstream).
		WithPersistence(persistence).
		Mock()

	user, err := local2.NewVoid()
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

	signals := make(chan api.Signal)
	actions := make(chan api.Action)

	exchange := local.NewExchange()
	statsProcessor := processor.MultiStats(exchange, user, signals)
	positionProcessor := processor.Position(exchange, user, actions)
	tradeProcessor := processor.Trade(exchange, user, actions, signals)
	for _, c := range model.Coins {
		err := overWatch.Start(c, exchange,
			statsProcessor,
			positionProcessor,
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
