package main

import (
	"context"
	"runtime"
	"time"

	"github.com/drakos74/free-coin/client/binance"
	"github.com/drakos74/free-coin/client/local"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/algo/processor/external"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/free-coin/internal/storage/file/json"
	cointime "github.com/drakos74/free-coin/internal/time"
	botlocal "github.com/drakos74/free-coin/user/local"
	"github.com/drakos74/free-coin/user/telegram"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

func main() {

	for {
		log.Info().Msg("running app ... ")
		// this should block ..
		run()
		time.Sleep(1 * time.Hour)
	}

}

func run() {
	ctx, cnl := context.WithCancel(context.Background())
	upstream := func(since int64) (api.Client, error) {
		return binance.NewClient(), nil
	}
	persistence := func(shard string) (storage.Persistence, error) {
		return json.NewJsonBlob("kline", shard, true), nil
	}
	client := local.NewClient(cointime.Range{
		From:    time.Now().Add(-7 * 24 * time.Hour),
		ToInt64: cointime.ToNano,
	}, uuid.New().String()).
		WithUpstream(upstream).
		WithPersistence(persistence)
	exchange := binance.NewExchange(binance.External)

	var user api.User
	var err error

	if runtime.GOOS == "darwin" {
		log.Warn().Msg("running local user interface")
		user, err = botlocal.NewUser("", "")
	} else {
		user, err = telegram.NewBot(api.External)
	}
	if err != nil {
		panic(err.Error())
	}

	err = user.Run(context.TODO())
	if err != nil {
		panic(err.Error())
	}

	overWatch := coin.New(client, user)
	finished := overWatch.Run(ctx)

	storageShard := json.BlobShard(storage.InternalPath)
	registry := json.NewEventRegistry(storage.SignalsPath)

	// load the default configuration
	configs := make(map[model.Coin]map[time.Duration]processor.Config)

	signal := external.MessageSignal{
		Output: make(chan external.Message),
	}

	signalProcessor := external.Signal("", storageShard, registry, exchange, user, signal, configs)

	// secondary user ...
	secSignal := external.MessageSignal{
		Source: signal.Output,
		Output: make(chan external.Message),
	}
	secExchange := local.Noop{}
	secSignalProcessor := external.Signal("Vagz", storageShard, registry, secExchange, user, secSignal, configs)

	for _, c := range model.Coins {
		if c != model.BTC {
			continue
		}
		err := overWatch.Start(c, coin.Log,
			signalProcessor,
			secSignalProcessor,
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
