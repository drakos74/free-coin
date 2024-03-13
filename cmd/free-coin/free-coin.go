package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/algo/processor/trade"

	"github.com/drakos74/free-coin/client/kraken"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/account"
	"github.com/drakos74/free-coin/internal/algo/processor/ml"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	json_storage "github.com/drakos74/free-coin/internal/storage/file/json"
	"github.com/drakos74/free-coin/user/telegram"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

const (
	exchange = "exchange"
	bot      = "bot"
)

func main() {
	//config := ml.Config(model.ETH)
	config := ml.Config()

	cc := make([]model.Coin, 0)
	coins := make(map[model.Coin]bool)
	for k, _ := range config.Segments {
		if !coins[k.Coin] {
			cc = append(cc, k.Coin)
			coins[k.Coin] = true
		}
	}

	// main engine trade input ...
	client := kraken.NewClient(cc...).
		//Since(cointime.LastXHours(48)).
		//Interval(2 * time.Second).
		Live(true)
	engine, err := coin.NewEngine(client)
	if err != nil {
		log.Fatalf("error creating engine: %s", err.Error())
	}

	// get config
	f := "free-coin.json"
	//f := "free-coin.json"
	dat, err := os.ReadFile(f)
	if err != nil {
		panic(any(fmt.Sprintf("could not load config file: %s", f)))
	}
	var cfg map[string]string
	err = json.Unmarshal(dat, &cfg)
	if err != nil {
		panic(any(fmt.Sprintf("could not decode config file: %s", f)))
	}

	// position tracker for kraken
	exchange := kraken.NewExchange(account.Name(cfg[exchange])) //account.Drakos
	u, err := telegram.NewBot(api.Index(cfg[bot]))              //api.FreeCoin
	if err != nil {
		log.Fatalf("error creating user: %s", err.Error())
	}
	//positionTracker := coin.NewStrategy("position-tracker").
	//	ForExchange(exchange).
	//	ForUser(u).
	//	WithProcessor(position.Processor(api.FreeCoin)).Apply()
	//engine.AddProcessor(positionTracker)

	shard := json_storage.BlobShard("ml")
	registry := json_storage.EventRegistry("ml-event-registry")
	//shard := json_storage.BlobShard("ml")
	//registry := json_storage.EventRegistry("ml-event-registry")
	strategy := processor.NewStrategy(config)
	engine.AddProcessor(coin.NewStrategy(trade.Name).
		ForUser(u).
		ForExchange(exchange).
		WithProcessor(trade.Processor(api.DrCoin, shard, registry, strategy)).
		Apply()).
		AddProcessor(coin.NewStrategy(ml.Name).
			ForUser(u).
			ForExchange(exchange).
			WithProcessor(ml.Processor(api.DrCoin, shard, strategy)).
			Apply())
	go u.Run(context.Background())
	err = engine.Run()
	if err != nil {
		log.Fatalf("error running engine: %s", err.Error())
	}
}
