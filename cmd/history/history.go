package main

import (
	"fmt"
	"log"
	"time"

	"github.com/drakos74/free-coin/client/kraken"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {

	cc := []model.Coin{model.SOL}

	// main engine trade input ...
	client := kraken.NewClient(cc...).
		Since(cointime.At(2020, 1, 1, 0)).
		Interval(2 * time.Second)
	engine, err := coin.NewEngine(client)
	if err != nil {
		log.Fatalf("error creating engine: %s", err.Error())
	}

	engine.AddProcessor(processor.History("history", model.ETH,
		fmt.Sprintf("%s/%s/%v", storage.DefaultDir, storage.HistoryDir, model.ETH)))

	err = engine.Run()
	if err != nil {
		log.Fatalf("error running engine: %s", err.Error())
	}
}
