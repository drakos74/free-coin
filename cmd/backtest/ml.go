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

		//rateW, err := strconv.ParseFloat(request.RateW[0], 64)
		//if err != nil {
		//	return []byte(err.Error()), http.StatusBadRequest, nil
		//}
		//
		//rateB, err := strconv.ParseFloat(request.RateB[0], 64)
		//if err != nil {
		//	return []byte(err.Error()), http.StatusBadRequest, nil
		//}

		mlModel := ""
		if len(request.Model) > 0 {
			mlModel = request.Model[0]
		}

		nn := configNN(0.1, 0.0)

		response := Response{
			Details: []Details{
				{
					Coin: string(requestCoin),
				},
			},
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

			cfg := configML(mlModel)

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

			signals := make([]*ml.Signal, 0)
			for _, m := range u.Messages {
				signal := new(ml.Signal)
				err := json.Unmarshal([]byte(m.Text), signal)
				if err != nil {
					fmt.Printf("msg err = %+v\n", err)
					continue
				}
				switch signal.Type {
				case model.Buy:
					response.Trigger.Buy = append(response.Trigger.Buy, Point{
						X: signal.Time,
						Y: signal.Price,
					})
				case model.Sell:
					response.Trigger.Sell = append(response.Trigger.Sell, Point{
						X: signal.Time,
						Y: signal.Price,
					})
				}
				response.Loss = append(response.Loss, Point{
					X: signal.Time,
					Y: float64(signal.Type),
				})
				signals = append(signals, signal)
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

func configML(m string) ml.Config {
	cfg := map[model.Coin]map[time.Duration]ml.Segments{
		model.BTC: {
			2 * time.Minute: ml.Segments{
				LookBack:  19,
				LookAhead: 1,
				Threshold: 0.5,
			},
			5 * time.Minute: ml.Segments{
				LookBack:  19,
				LookAhead: 1,
				Threshold: 0.5,
			},
			15 * time.Minute: ml.Segments{
				LookBack:  19,
				LookAhead: 1,
				Threshold: 0.75,
			},
			30 * time.Minute: ml.Segments{
				LookBack:  9,
				LookAhead: 1,
				Threshold: 1,
			},
			60 * time.Minute: ml.Segments{
				LookBack:  9,
				LookAhead: 1,
				Threshold: 1,
			},
			120 * time.Minute: ml.Segments{
				LookBack:  4,
				LookAhead: 1,
				Threshold: 1,
			},
		},
	}

	if m != "" {
		p := strings.Split(m, "_")
		f, _ := strconv.ParseFloat(p[2], 64)
		t, _ := time.ParseDuration(p[1])
		cc := ModelParser{
			Coin:      model.Coin(p[0]),
			Threshold: f,
			Duration:  t,
		}

		cfg = map[model.Coin]map[time.Duration]ml.Segments{
			cc.Coin: {
				cc.Duration: cfg[cc.Coin][cc.Duration],
			},
		}

		segment := cfg[cc.Coin][cc.Duration]
		segment.Model = m
		cfg[cc.Coin][cc.Duration] = segment
	}

	return cfg
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
