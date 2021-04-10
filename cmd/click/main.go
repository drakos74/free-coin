package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/drakos74/free-coin/internal/account"

	"github.com/drakos74/free-coin/client/binance"
	"github.com/drakos74/free-coin/client/local"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/algo/processor/signal"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/free-coin/internal/storage/file/json"
	cointime "github.com/drakos74/free-coin/internal/time"
	botlocal "github.com/drakos74/free-coin/user/local"
	"github.com/drakos74/free-coin/user/telegram"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

func main() {
	for {
		logger.Info().Msg("running app ... ")
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
	exchange := binance.NewExchange(account.Parakmi)
	//exchange := local.NewExchange("coin_click_exchange")

	var user api.User
	var err error

	if runtime.GOOS == "darwin" {
		logger.Warn().Msg("running local user interface")
		user, err = botlocal.NewUser("coin_click_bot")
	} else {
		user, err = telegram.NewBot(api.CoinClick)
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

	details := []account.Details{
		{
			Name: account.Parakmi,
			//Exchange: account.ExchangeDetails{
			//	Name: binance.Name,
			//},
			User: account.UserDetails{
				Index: api.CoinClick,
				Alias: "moneytized",
			},
		},
		{
			Name: account.Drakos,
			Exchange: account.ExchangeDetails{
				Name:   binance.Name,
				Margin: true,
			},
			User: account.UserDetails{
				Index: api.CoinClick,
				Alias: "Vagz",
			},
		},
	}

	signalChannel := signal.MessageSignal{
		Output: make(chan signal.Message),
	}
	output := make(chan signal.Message)

	processors := make([]api.Processor, 0)

	processors = append(processors, signal.Receiver("", storageShard, registry, exchange, user, signalChannel, configs))
	// + add the default user for the processor to be able to reply
	err = user.AddUser(api.CoinClick, "")
	if err != nil {
		log.Fatalf(err.Error())
	}

	for _, detail := range details {

		if detail.User.Index != "" && detail.User.Alias != "" {
			// add the users
			err = user.AddUser(detail.User.Index, detail.User.Alias)
			if err != nil {
				log.Fatalf(err.Error())
			}
		} else {
			logger.Warn().Str("user", string(detail.Name)).Msg("user has no comm channel config")
		}

		if detail.Exchange.Name != "" {
			// secondary user ...
			userSignal := signal.MessageSignal{
				Source: signalChannel.Output,
				Output: make(chan signal.Message),
			}
			var userExchange api.Exchange
			if detail.Exchange.Margin {
				userExchange = binance.NewMarginExchange(detail.Name)
			} else {
				userExchange = binance.NewExchange(detail.Name)
			}
			processors = append(processors, signal.Receiver(detail.User.Alias, storageShard, registry, userExchange, user, userSignal, configs))
			output = userSignal.Output
			logger.Info().
				Str("exchange", string(detail.Exchange.Name)).
				Bool("margin", detail.Exchange.Margin).
				Str("user", string(detail.Name)).
				Msg("init exchange")
		} else {
			logger.Warn().
				Str("exchange", string(detail.Exchange.Name)).
				Bool("margin", detail.Exchange.Margin).
				Str("user", string(detail.Name)).
				Msg("user has not exchange config")
		}

	}

	// TODO : orchestrate the closing of signals

	// add a final processor for the signals ...
	go func() {
		for msg := range output {
			logger.Debug().Str("message", fmt.Sprintf("%+v", msg)).Msg("signal received")
		}
	}()

	for _, c := range model.Coins {
		if c != model.BTC {
			continue
		}
		err := overWatch.Start(c, coin.Log,
			processors...,
		)
		if err != nil {
			logger.Error().Str("coin", string(c)).Err(err).Msg("could not start engine")
		}
	}

	// this is a long running task ... lets keep the main thread occupied
	// until we get a signal from the overwatch
	finished.Wait()
	cnl()
}
