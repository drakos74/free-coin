package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	model2 "github.com/drakos74/free-coin/internal/algo/processor/ml/model"

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
		var pp = make(map[string]float64)
		err := filepath.Walk("file-storage/ml/models", func(path string, info fs.FileInfo, err error) error {
			if !info.IsDir() {
				p := filepath.Base(path)
				prefix := strings.Split(p, model.Delimiter)
				if len(prefix) > 1 {
					pre := fmt.Sprintf("%s-%s", prefix[0], prefix[1])
					var prec = 0.0
					if len(prefix) > 5 {
						prec, err = strconv.ParseFloat(prefix[5], 64)
					}
					if f, ok := pp[pre]; ok {
						if f < prec {
							pp[pre] = prec
							files = append(files, filepath.Base(path))
						}
					} else {
						pp[pre] = prec
						files = append(files, filepath.Base(path))
					}
				}
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

		//delete the internal positions file
		err := os.Remove("file-storage/ml/trader/free_coin/all_0_free_coin.json")
		//if err != nil {
		//	return []byte(err.Error()), http.StatusInternalServerError, nil
		//}

		//err = os.WriteFile("file-storage/ml/trader/free_coin/all_0_free_coin.json", []byte("{}"), 0644)
		//if err != nil {
		//	return []byte(err.Error()), http.StatusInternalServerError, nil
		//}

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

		lookBack, err := strconv.Atoi(request.LookBack[0])
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		lookAhead, err := strconv.Atoi(request.LookAhead[0])
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		gap, err := strconv.ParseFloat(request.Gap[0], 64)
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

		features, err := strconv.Atoi(request.Features[0])
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}
		//
		//rateB, err := strconv.ParseFloat(request.RateB[0], 64)
		//if err != nil {
		//	return []byte(err.Error()), http.StatusBadRequest, nil
		//}

		bufferTime, err := strconv.ParseFloat(request.BufferTime[0], 64)
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		PriceThreshold, err := strconv.ParseFloat(request.PriceThreshold[0], 64)
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		stopLoss, err := strconv.ParseFloat(request.StopLoss[0], 64)
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		takeProfit, err := strconv.ParseFloat(request.TakeProfit[0], 64)
		if err != nil {
			return []byte(err.Error()), http.StatusBadRequest, nil
		}

		mm := make(map[model.Key]model2.Segments)

		for _, m := range request.Model {
			// TODO : note this needs to be the hash delimiter of the model.Key
			p := strings.Split(m, model.Delimiter)
			c := model.Coin(p[0])

			if c != requestCoin {
				return nil, 0, fmt.Errorf("invalid coin model")
			}

			d, err := strconv.Atoi(p[1])
			if err != nil {
				return []byte(err.Error()), http.StatusBadRequest, nil
			}
			duration := time.Duration(d) * time.Minute
			k := model.Key{
				Coin:     c,
				Duration: duration,
				Strategy: "ui",
			}
			mm[k] = model2.Segments{
				Stats: model2.Stats{
					LookBack:  lookBack,
					LookAhead: lookAhead,
					Gap:       gap,
				},
				Model: model2.Model{
					BufferSize:         bufferSize,
					PrecisionThreshold: precision,
					ModelSize:          size,
					Features:           features,
				},
				Trader: model2.Trader{
					BufferTime:     bufferTime,
					PriceThreshold: PriceThreshold,
					Weight:         1,
				},
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
			registry := storage.EventRegistry("ml-trade-registry")
			cfg := configML(mm, takeProfit, stopLoss)

			network := coin.NewStrategy(ml.Name).
				ForUser(u).
				ForExchange(exchange).
				WithProcessor(ml.Processor(api.FreeCoin, shard, registry, nn, &cfg)).Apply()

			// reduce data size for viewing purposes
			minutes := to.Sub(from).Minutes()
			d := int(minutes / 1000)
			window := buffer.NewHistoryWindow(time.Minute*time.Duration(d), 1)

			tradeTracker := coin.NewStrategy("Trades").
				ForUser(u).
				WithProcessor(func(u api.User, e api.Exchange) api.Processor {
					return processor.Process("Trades", func(trade *model.TradeSignal) error {
						if b, ok := window.Push(trade.Time, trade.Tick.Price); ok {
							price := b.Bucket.Values().Stats()[0].Avg()
							func(trade model.TradeSignal, price float64) {
								response.Trades = append(response.Trades, Point{
									X: trade.Time,
									Y: trade.Tick.Price,
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

			signals := make(map[string][]*model2.Signal, 0)
			for _, m := range u.Messages {
				fmt.Printf("m = %+v\n", m)
				signal := new(model2.Signal)
				err := json.Unmarshal([]byte(m.Text), signal)
				if err != nil {
					fmt.Printf("msg err = %+v\n", err)
					continue
				}

				key := model.Key{
					Coin:     signal.Key.Coin,
					Duration: signal.Key.Duration,
				}
				if signal.Key.Coin != "" {
					key = signal.Key
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
					signals[key.ToString()] = make([]*model2.Signal, 0)
				}
				ss := append(signals[key.ToString()], signal)
				signals[key.ToString()] = ss
			}
		}

		report := exchange.Gather(true)
		response.Report = report

		orders := exchange.Orders()
		for _, order := range orders {
			//fmt.Printf("time = %+v , price = %+v , type = %+v\nreason = %+v\n", order.Time.Format(time.Stamp), order.Price, order.Type, order.Reason)
			key := model.Key{
				Coin:  requestCoin,
				Index: -1,
			}

			if _, ok := response.Trigger[key.ToString()]; !ok {
				response.Trigger[key.ToString()] = Trigger{
					Buy:  make([]Point, 0),
					Sell: make([]Point, 0),
				}
			}

			var buy = response.Trigger[key.ToString()].Buy
			var sell = response.Trigger[key.ToString()].Sell

			point := Point{
				X: order.Time,
				Y: order.Price,
			}

			switch order.Type {
			case model.Buy:
				buy = append(buy, point)
			case model.Sell:
				sell = append(sell, point)
			}
			response.Trigger[key.ToString()] = Trigger{
				Buy:  buy,
				Sell: sell,
			}
		}

		bb, err := json.Marshal(response)
		if err != nil {
			return []byte(err.Error()), http.StatusInternalServerError, nil
		}

		return bb, http.StatusOK, nil
	}
}

func configML(mm map[model.Key]model2.Segments, tp, sl float64) model2.Config {
	cfg := map[model.Key]model2.Segments{
		model.Key{
			Coin:     model.BTC,
			Duration: 2 * time.Minute,
			Strategy: "default",
		}: {
			Stats: model2.Stats{
				LookBack:  9,
				LookAhead: 1,
				Gap:       0.5,
			},
		},
		model.Key{
			Coin:     model.BTC,
			Duration: 5 * time.Minute,
			Strategy: "default",
		}: {
			Stats: model2.Stats{
				LookBack:  9,
				LookAhead: 1,
				Gap:       0.5,
			},
		},
		model.Key{
			Coin:     model.BTC,
			Duration: 15 * time.Minute,
			Strategy: "default",
		}: {
			Stats: model2.Stats{
				LookBack:  9,
				LookAhead: 1,
				Gap:       0.5,
			},
		},
		model.Key{
			Coin:     model.BTC,
			Duration: 30 * time.Minute,
			Strategy: "default",
		}: {
			Stats: model2.Stats{
				LookBack:  9,
				LookAhead: 1,
				Gap:       0.5,
			},
		},
		model.Key{
			Coin:     model.BTC,
			Duration: 60 * time.Minute,
			Strategy: "default",
		}: {
			Stats: model2.Stats{
				LookBack:  9,
				LookAhead: 1,
				Gap:       0.5,
			},
		},
		model.Key{
			Coin:     model.BTC,
			Duration: 240 * time.Minute,
			Strategy: "default",
		}: {
			Stats: model2.Stats{
				LookBack:  9,
				LookAhead: 1,
				Gap:       0.5,
			},
		},
	}

	if len(mm) > 0 {
		cfg = mm
	}

	return model2.Config{
		Segments: cfg,
		Position: model2.Position{
			OpenValue:  1000,
			StopLoss:   tp,
			TakeProfit: sl,
		},
		Option: model2.Option{
			Debug:     true,
			Benchmark: true,
		},
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
