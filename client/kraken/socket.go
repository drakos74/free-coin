package kraken

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	ws "github.com/aopoltorzhicky/go_kraken/websocket"
)

func Run() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	kraken := ws.NewKraken(ws.ProdBaseURL)
	if err := kraken.Connect(); err != nil {
		log.Fatalf("Error connecting to web socket: %s", err.Error())
	}

	// subscribe to BTCUSD`s ticker
	if err := kraken.SubscribeTicker([]string{ws.BTCEUR}); err != nil {
		log.Fatalf("SubscribeTicker error: %s", err.Error())
	}

	//if err := kraken.SubscribeBook([]string{ws.BTCEUR}, 10); err != nil {
	//	log.Fatalf("SubscribeBook error: %s", err.Error())
	//}

	if err := kraken.SubscribeSpread([]string{ws.BTCEUR}); err != nil {
		log.Fatalf("SubscribeTrades error: %s", err.Error())
	}

	for {
		select {
		case <-signals:
			log.Print("Stopping...")
			if err := kraken.Close(); err != nil {
				log.Fatal(err)
			}
			return
		case update := <-kraken.Listen():
			switch data := update.Data.(type) {
			case ws.TickerUpdate:
				log.Printf("----Ticker of %s----", update.Pair)
				log.Printf("Ask: %s with %s", data.Ask.Price.String(), data.Ask.Volume.String())
				log.Printf("Bid: %s with %s", data.Bid.Price.String(), data.Bid.Volume.String())
				log.Printf("Open today: %s | Open last 24 hours: %s", data.Open.Today.String(), data.Open.Last24.String())
			default:
				fmt.Printf("data = %+v\n", data)
			}
		}
	}
}
