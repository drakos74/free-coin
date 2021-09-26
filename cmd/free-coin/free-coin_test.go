package main

import (
	"log"
	"testing"
	"time"

	"github.com/drakos74/free-coin/client/kraken"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
)

func TestFreeCoin(t *testing.T) {
	limit := 7999
	client := kraken.NewClient(model.BTC, model.ETH).
		Interval(1 * time.Second).
		Stop(api.Counter(limit)).
		WithRemote(kraken.NewMockSource("../../client/kraken/testdata/response-trades"))

	engine, err := coin.NewEngine(client)
	if err != nil {
		log.Fatalf("error creating engine: %s", err.Error())
	}

	err = engine.Run()
	if err != nil {
		log.Fatalf("error running engine: %s", err.Error())
	}
}
