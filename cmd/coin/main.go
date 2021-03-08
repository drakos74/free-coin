package main

import (
	"context"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"

	"github.com/drakos74/free-coin/user/telegram"

	"github.com/drakos74/free-coin/client/kraken"
	"github.com/drakos74/free-coin/client/local"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/algo/processor/position"
	"github.com/drakos74/free-coin/internal/algo/processor/stats"
	"github.com/drakos74/free-coin/internal/algo/processor/trade"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/free-coin/internal/storage/file/json"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {

	ctx, cnl := context.WithCancel(context.Background())
	upstream := func(since int64) (api.Client, error) {
		return kraken.NewClient(since, 15*time.Second, api.NonStop), nil
	}
	persistence := func(shard string) (storage.Persistence, error) {
		return json.NewJsonBlob("trades", shard), nil
	}
	client := local.NewClient(cointime.Range{
		From:    time.Now().Add(-7 * 24 * time.Hour),
		ToInt64: cointime.ToNano,
	}, uuid.New().String()).
		WithUpstream(upstream).
		WithPersistence(persistence)
	//client := kraken.NewClient(ctx, cointime.LastXHours(99), 10*time.Second)

	user, err := telegram.NewBot()
	//user, err := botlocal.NewUser("", "")
	if err != nil {
		panic(err.Error())
	}

	err = user.Run(context.TODO())
	if err != nil {
		panic(err.Error())
	}

	overWatch := coin.New(client, user)
	finished := overWatch.Run(ctx)

	registry := json.NewEventRegistry(storage.RegistryPath)

	exchange := kraken.NewExchange(ctx)
	block := api.NewBlock()

	// load the default configuration
	configs := processor.LoadDefaults(model.Coins)
	storageShard := json.BlobShard(storage.InternalPath)
	statsProcessor := stats.MultiStats(registry, user, configs)
	positionProcessor := position.Position(storageShard, registry, exchange, user, block, configs)
	tradeProcessor := trade.Trade(registry, user, block, configs)

	for _, c := range model.Coins {
		err := overWatch.Start(c, coin.Log,
			statsProcessor,
			positionProcessor,
			tradeProcessor,
		)
		if err != nil {
			log.Error().Str("coin", string(c)).Err(err).Msg("could not start engine")
		}
	}

	// this is a long running task ... lets keep the main thread occupied
	// until we get a signal from the overwatch
	finished.Wait()
	cnl()
}
