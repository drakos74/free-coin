package signal

import (
	"fmt"
	"log"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor/signal/processor"

	"github.com/drakos74/free-coin/client/binance"
	"github.com/drakos74/free-coin/internal/algo/processor/signal/http"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/free-coin/internal/storage/file/json"
	logger "github.com/rs/zerolog/log"
)

func New() func(u api.User, e api.Exchange) api.Processor {

	accounts := getAccounts()

	userSignal := processor.NewSignalChannel(nil)

	processors := make([]api.Processor, 0)

	storageShard := json.BlobShard(storage.InternalPath)
	registry := json.EventRegistry(storage.SignalsPath)

	return func(u api.User, e api.Exchange) api.Processor {

		// load the default configuration
		configs := make(map[model.Coin]map[time.Duration]processor.Settings)

		processors = append(processors, http.Propagate(registry, e, u, userSignal.Output))
		// + add the default user for the processor to be able to reply
		err := u.AddUser(api.CoinClick, "", 0)
		if err != nil {
			log.Fatalf(err.Error())
		}

		output := make(chan processor.Message)
		for _, detail := range accounts {

			if detail.User.Index != "" && detail.User.Alias != "" {
				// add the users
				err = u.AddUser(detail.User.Index, detail.User.Alias, detail.User.ChatID)
				if err != nil {
					log.Fatalf(err.Error())
				}
			} else {
				logger.Warn().Str("user", string(detail.Name)).Msg("user has no comm channel config")
			}

			if detail.Exchange.Name != "" {
				// secondary user ...
				userSignal = processor.NewSignalChannel(userSignal.Output)
				var userExchange api.Exchange
				if detail.Exchange.Margin {
					userExchange = binance.NewMarginExchange(detail.Name)
				} else {
					userExchange = binance.NewExchange(detail.Name)
				}
				processors = append(processors, processor.Receiver(detail.User.Alias, storageShard, registry, userExchange, u, userSignal, configs))
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

		// enable the pipeline
		go func() {
			for msg := range output {
				logger.Debug().Str("message", fmt.Sprintf("%+v", msg)).Msg("signal processed")
			}
			logger.Info().Msg("closed signal processor")
		}()

		return func(in <-chan *model.Trade, out chan<- *model.Trade) {
			// TODO : Nothing to do here with the trades ... for now
			for trade := range in {
				out <- trade
			}
		}

	}

}
