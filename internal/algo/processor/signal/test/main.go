package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/drakos74/free-coin/internal/account"
	"github.com/drakos74/free-coin/user/telegram"

	botlocal "github.com/drakos74/free-coin/user/local"

	"github.com/drakos74/free-coin/client/binance"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/free-coin/internal/storage/file/json"

	"github.com/rs/zerolog"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
}

func main() {

	// telegram.NewBot(api.CoinClick)
	user, err := botlocal.NewUser("coin_click_bot")

	testAccount := account.Details{
		Name: account.Drakos,
		Exchange: account.ExchangeDetails{
			Name:   binance.Name,
			Margin: true,
		},
		User: account.UserDetails{
			Index:  api.CoinClick,
			Alias:  "Vagz",
			ChatID: telegram.CoinClickChatID,
		},
	}

	go app.New(testAccount).
		Upstream(func(since int64) (api.Client, error) {
			return binance.NewClient(), nil
		}).
		History(func(shard string) (storage.Persistence, error) {
			return json.NewJsonBlob("kline", shard, true), nil
		}).
		User(user).
		Storage(json.BlobShard(storage.InternalPath)).
		Registry(json.EventRegistry(storage.SignalsPath)).
		Run()

	time.Sleep(10 * time.Second)
	// start sending a few trades
	jsonStr, err := ioutil.ReadFile("cmd/click/test/testdata/sample_message.json")

	fmt.Printf("jsonStr = %+v\n", jsonStr)
	req, err := http.NewRequest("POST", "http://localhost:8080/api/test-post", bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	fmt.Printf("resp = %+v\n", resp)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// wait forever (?)
	time.Sleep(100 * time.Second)

}
