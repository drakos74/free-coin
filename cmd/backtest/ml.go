package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/trader"

	"github.com/drakos74/free-coin/client/history"
	localExchange "github.com/drakos74/free-coin/client/local"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/algo/processor/ml"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/server"
	storage "github.com/drakos74/free-coin/internal/storage/file/json"
	"github.com/drakos74/free-coin/user/local"
	xml "github.com/drakos74/go-ex-machina/xmachina/ml"
	"github.com/drakos74/go-ex-machina/xmachina/net"
	"github.com/drakos74/go-ex-machina/xmachina/net/ff"
	"github.com/drakos74/go-ex-machina/xmath"
	"github.com/rs/zerolog/log"
)

func models() server.Handler {
	return func(ctx context.Context, r *http.Request) ([]byte, int, error) {

		var files []string

		err := filepath.Walk("file-storage/ml/models", func(path string, info fs.FileInfo, err error) error {
			if !info.IsDir() {
				files = append(files, filepath.Base(path))
			}
			return nil
		})

		if err != nil {
			return nil, http.StatusInternalServerError, nil
		}

		rsp, _ := json.Marshal(files)
		return rsp, http.StatusOK, nil
	}
}

func train() server.Handler {
	return func(ctx context.Context, r *http.Request) ([]byte, int, error) {
		request, err := parseTrain(r.URL.Query())
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		requestCoin := model.Coin(request.Coin[0])
		from, err := time.Parse("2006_01_02T15", request.From[0])
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}
		to, err := time.Parse("2006_01_02T15", request.To[0])
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		precision, err := strconv.ParseFloat(request.Precision[0], 64)
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		size, err := strconv.Atoi(request.Size[0])
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		bufferSize, err := strconv.Atoi(request.BufferSize[0])
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}
		//
		//rateB, err := strconv.ParseFloat(request.RateB[0], 64)
		//if err != nil {
		//	return []byte(err.Error()), http.StatusBadRequest, nil
		//}

		mm := make(map[model.Coin]map[time.Duration]ml.Segments)

		for _, m := range request.Model {
			p := strings.Split(m, "_")
			fmt.Printf("p = %+v\n", p)
			c := model.Coin(p[0])
			d, err := time.ParseDuration(p[1])
			if err != nil {
				log.Error().Err(err).Msg("could not parse duration")
				return []byte(err.Error()), http.StatusBadRequest, nil
			}
			if _, ok := mm[c]; !ok {
				mm[c] = make(map[time.Duration]ml.Segments)
			}
			if _, ok := mm[c][d]; !ok {
				mm[c][d] = ml.Segments{}
			}
		}

		//mlModel := ""
		//if len(request.Model) > 0 {
		//	mlModel = request.Model[0]
		//}

		nn := configNN(0.1, 0.0)

		response := Response{
			Details: []Details{
				{
					Coin: string(requestCoin),
				},
			},
			Trigger: make(map[string]Trigger),
		}

		var u *local.User
		var exchange *localExchange.Exchange

		for i := 0; i < 1; i++ {

			response = Response{
				Details: []Details{
					{
						Coin: string(requestCoin),
					},
				},
				Trigger: make(map[string]Trigger),
			}

			u, err = local.NewUser("user.log")
			exchange = localExchange.NewExchange("exchange.log")

			client := history.New(nil).Reader(&history.Request{
				Coin: requestCoin,
				From: from,
				To:   to,
			}).WithRegistry(storage.NewEventRegistry("Trades"))

			engine, err := coin.NewEngine(client)
			if err != nil {
				return []byte(err.Error()), http.StatusInternalServerError, nil
			}

			shard := storage.BlobShard("ml")

			cfg := configML(mm)

			cfg.Model.Threshold = precision
			cfg.Model.BufferSize = bufferSize
			cfg.Model.Size = size
			cfg.Model.Features = 3

			network := coin.NewStrategy(ml.Name).
				ForUser(u).
				ForExchange(exchange).
				WithProcessor(ml.Processor(api.FreeCoin, shard, nn, cfg)).Apply()

			// reduce data size for viewing purposes
			minutes := to.Sub(from).Minutes()
			d := int(minutes / 1000)
			window := buffer.NewHistoryWindow(time.Minute*time.Duration(d), 1)

			tradeTracker := coin.NewStrategy("Trades").
				ForUser(u).
				WithProcessor(func(u api.User, e api.Exchange) api.Processor {
					return processor.Process("Trades", func(trade *model.Trade) error {
						if b, ok := window.Push(trade.Time, trade.Price); ok {
							price := b.Bucket.Values().Stats()[0].Avg()
							func(trade model.Trade, price float64) {
								response.Trades = append(response.Trades, Point{
									X: trade.Time,
									Y: trade.Price,
								})
								response.Time = append(response.Time, trade.Time)
								response.Price = append(response.Price, Point{
									X: trade.Time,
									Y: price,
								})
							}(*trade, price)
						}
						exchange.Process(trade)
						//u.Send(api.DrCoin, api.NewMessage(fmt.Sprintf("%v:%s:%4.f", trade.Time, trade.Coin, trade.Price)), nil)
						return nil
					})
				}).Apply()

			engine.
				//AddProcessor(positionTracker).
				AddProcessor(network).
				AddProcessor(tradeTracker)

			err = engine.Run()

			// gather all signals for different scenarios
			log.Info().Int("count", len(u.Messages)).Msg("messages")

			signals := make(map[string][]*ml.Signal, 0)
			for _, m := range u.Messages {
				signal := new(ml.Signal)
				err := json.Unmarshal([]byte(m.Text), signal)
				if err != nil {
					fmt.Printf("msg err = %+v\n", err)
					continue
				}

				key := trader.Key{
					Coin:     signal.Coin,
					Duration: signal.Duration,
				}

				if _, ok := response.Trigger[key.ToString()]; !ok {
					response.Trigger[key.ToString()] = Trigger{
						Buy:  make([]Point, 0),
						Sell: make([]Point, 0),
					}
				}

				var buy = response.Trigger[key.ToString()].Buy
				var sell = response.Trigger[key.ToString()].Sell

				switch signal.Type {
				case model.Buy:
					buy = append(buy, Point{
						X: signal.Time,
						Y: signal.Price,
					})
				case model.Sell:
					sell = append(sell, Point{
						X: signal.Time,
						Y: signal.Price,
					})
				}
				response.Loss = append(response.Loss, Point{
					X: signal.Time,
					Y: float64(signal.Type),
				})
				response.Trigger[key.ToString()] = Trigger{
					Buy:  buy,
					Sell: sell,
				}
				if _, ok := signals[key.ToString()]; !ok {
					signals[key.ToString()] = make([]*ml.Signal, 0)
				}
				ss := append(signals[key.ToString()], signal)
				signals[key.ToString()] = ss
			}
		}

		exchange.Gather()

		bb, err := json.Marshal(response)
		if err != nil {
			return []byte(err.Error()), http.StatusInternalServerError, nil
		}

		return bb, http.StatusOK, nil
	}
}

func configML(mm map[model.Coin]map[time.Duration]ml.Segments) ml.Config {
	cfg := map[model.Coin]map[time.Duration]ml.Segments{
		model.BTC: {
			2 * time.Minute: ml.Segments{
				LookBack:  9,
				LookAhead: 1,
				Threshold: 0.5,
				Model:     "2",
			},
			5 * time.Minute: ml.Segments{
				LookBack:  9,
				LookAhead: 1,
				Threshold: 0.5,
				Model:     "5",
			},
			15 * time.Minute: ml.Segments{
				LookBack:  9,
				LookAhead: 1,
				Threshold: 0.75,
				Model:     "15",
			},
			30 * time.Minute: ml.Segments{
				LookBack:  9,
				LookAhead: 1,
				Threshold: 1,
				Model:     "30",
			},
			60 * time.Minute: ml.Segments{
				LookBack:  9,
				LookAhead: 1,
				Threshold: 1.5,
				Model:     "60",
			},
			240 * time.Minute: ml.Segments{
				LookBack:  9,
				LookAhead: 1,
				Threshold: 2,
				Model:     "240",
			},
		},
	}

	if len(mm) > 0 {
		for c, md := range mm {
			if _, ok := cfg[c]; ok {
				for d, _ := range md {
					if _, ok := cfg[c][d]; ok {
						mm[c][d] = cfg[c][d]
					}
				}
			}
		}
	}

	return ml.Config{
		Segments:  mm,
		Debug:     true,
		Benchmark: true,
	}
}

type ModelParser struct {
	Coin      model.Coin
	Threshold float64
	Duration  time.Duration
}

func configNN(rateW, rateB float64) *ff.Network {
	// build the network
	rate := xml.Learn(rateW, rateB)
	initW := xmath.Rand(0, 1, math.Sqrt)
	initB := xmath.Rand(0, 1, math.Sqrt)
	nn := ff.New(10, 1).
		Add(60, net.NewBuilder().
			WithModule(xml.Base().
				WithRate(rate).
				WithActivation(xml.TanH)).
			WithWeights(initW, initB).
			Factory(net.NewActivationCell)).
		Add(10, net.NewBuilder().
			WithModule(xml.Base().
				WithRate(rate).
				WithActivation(xml.TanH)).
			WithWeights(initW, initB).
			Factory(net.NewActivationCell)).
		Add(3, net.NewBuilder().
			WithModule(xml.Base().
				WithRate(rate).
				WithActivation(xml.TanH)).
			WithWeights(initW, initB).
			Factory(net.NewActivationCell)).
		Add(3, net.NewBuilder().CellFactory(net.NewSoftCell))
	nn.Loss(xml.Pow)
	nn.Debug()
	return nn
}
