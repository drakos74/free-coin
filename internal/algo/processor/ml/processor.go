package ml

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	coin_math "github.com/drakos74/free-coin/internal/math"

	"github.com/drakos74/free-coin/client"
	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/math/ml"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/free-coin/internal/trader"
	"github.com/drakos74/go-ex-machina/xmachina/net/ff"
	"github.com/rs/zerolog/log"
)

const (
	Name = "ml-network"
)

// Processor is the position processor main routine.
func Processor(index api.Index, shard storage.Shard, _ *ff.Network, config Config) func(u api.User, e api.Exchange) api.Processor {
	collector, err := newCollector(shard, nil, config)
	// make sure we dont break the pipeline
	if err != nil {
		log.Error().Err(err).Str("processor", Name).Msg("could not init processor")
		return func(u api.User, e api.Exchange) api.Processor {
			return processor.Void(Name)
		}
	}

	//network := math.NewML(net)

	benchmarks := newBenchmarks()

	strategy := newStrategy(config.Segments)
	return func(u api.User, e api.Exchange) api.Processor {
		wallet, err := trader.SimpleTrader(string(index), shard, make(map[model.Coin]map[time.Duration]trader.Settings), e)
		if err != nil {
			log.Error().Err(err).Str("processor", Name).Msg("processor in void state")
			return processor.NoProcess(Name)
		}

		go trackUserActions(index, u, collector, strategy, wallet)

		dts := make(map[model.Coin]map[time.Duration]dataset)

		return processor.ProcessWithClose(Name, func(trade *model.Trade) error {
			metrics.Observer.IncrementTrades(string(trade.Coin), Name)
			signals := make(map[time.Duration]Signal)
			if vec, ok := collector.push(trade); ok {
				for d, vv := range vec {
					metrics.Observer.IncrementEvents(string(trade.Coin), d.String(), "poly", Name)
					configSegments := config.segments(trade.Coin, d)
					for key, segmentConfig := range configSegments {
						if _, ok := dts[key.Coin]; !ok {
							dts[key.Coin] = make(map[time.Duration]dataset)
						}
						if _, ok := dts[key.Coin][key.Duration]; !ok {
							dts[key.Coin][key.Duration] = newDataSet(trade.Coin, d, segmentConfig, make([]vector, 0))
						}
						newVectors := append(dts[key.Coin][key.Duration].vectors, vv)
						s := len(newVectors)
						if s > segmentConfig.Model.BufferSize {
							newVectors = newVectors[s-segmentConfig.Model.BufferSize:]
						}
						dts[key.Coin][key.Duration] = newDataSet(trade.Coin, d, segmentConfig, newVectors)
						// do our training here ...
						if segmentConfig.Model.Features > 0 {
							metrics.Observer.IncrementEvents(string(trade.Coin), d.String(), "train", Name)
							if len(dts[key.Coin][key.Duration].vectors) >= segmentConfig.Model.BufferSize {
								metrics.Observer.IncrementEvents(string(trade.Coin), d.String(), "train_buffer", Name)
								prec, err := dts[key.Coin][key.Duration].fit(segmentConfig.Model, false)
								if err != nil {
									log.Error().Err(err).Msg("could not train online")
								} else if prec > segmentConfig.Model.PrecisionThreshold {
									metrics.Observer.IncrementEvents(string(trade.Coin), d.String(), "train_threshold", Name)
									t := dts[key.Coin][key.Duration].predict(segmentConfig.Model)
									if t == model.NoType {
										log.Debug().Str("set", fmt.Sprintf("%+v", dts[key.Coin][key.Duration])).Str("type", t.String()).Msg("no consistent prediction")
										continue
									}
									signal := Signal{
										Key:       key,
										Coin:      trade.Coin,
										Time:      trade.Time,
										Duration:  d,
										Price:     trade.Price,
										Type:      t,
										Precision: prec,
										Spectrum:  coin_math.FFT(vv.yy),
										Buffer:    vv.yy,
									}
									signals[d] = signal
									if config.Debug {
										// TODO : make the exchange call on the above type
									}
									if config.Benchmark {
										report, ok, err := benchmarks.add(key, trade, signal, config)
										if err != nil {
											log.Error().Err(err).Msg("could not submit benchmark")
										} else if ok {
											strategy.report[key] = report
											if config.Debug {
												// for benchmark during backtest
												//u.Send(index, api.NewMessage(encodeMessage(signal)), nil)
											} else {
												// for live trading info
												//if cointime.IsValidTime(trade.Time) {
												//	u.Send(index, api.NewMessage(formatSignal(signal)).AddLine(formatReport(report)), nil)
												//}
											}
										}
									}
								}
							}
						}
					}
				}
				// NOTE : any real trading happens below this point
				if !config.Debug && !strategy.isLive(trade) {
					return nil
				}
				if len(signals) > 0 {
					if !config.Debug {
						u.Send(index, api.NewMessage(formatSignals(signals)), nil)
					}
					// TODO : decide how to make a unified trading strategy for the real trading
					var signal Signal
					var act bool
					for _, s := range signals {
						if signal.Type == model.NoType {
							signal = s
							act = true
						} else if signal.Type != s.Type || signal.Coin != s.Coin {
							act = false
						}
					}
					// TODO : get buy or sell from combination of signals
					signal.Duration = 0
					signal.Weight = len(signals)
					k := model.Key{
						Coin:     signal.Coin,
						Duration: signal.Duration,
						Strategy: "wallet",
					}
					signal.Key = k
					// if we get the go-ahead from the strategy act on it
					if act {
						strategy.signal(k, signal)
					} else {
						log.Debug().
							Str("signals", fmt.Sprintf("%+v", signals)).
							Msg("ignoring signals")
					}
				}
				if s, k, open, ok := strategy.trade(trade); ok {
					// TODO : fix this add-hoc number
					//if s.Spectrum.Mean() > 100 {
					//	open = false
					//}
					ok, err := submitTrade(index, k, s, wallet, u, open, config)
					if err != nil || !ok {
						log.Error().Err(err).Bool("ok", ok).Msg("could not submit trade")
					} else {
						// log the action

					}
				}
			}
			pp, profit := wallet.CheckPosition(model.Key{Coin: trade.Coin}, trade.Price, config.Position.TakeProfit, config.Position.StopLoss)
			if len(pp) > 0 {
				for k, p := range pp {
					_, ok, action, err := wallet.CreateOrder(k, trade.Time, trade.Price, p.Type.Inv(), false, p.Volume)
					if err != nil || !ok {
						log.Error().Err(err).Bool("ok", ok).Msg("could not close position")
					} else if profit < 0 {
						ok := strategy.reset(k)
						if !ok {
							log.Error().Str("key", k.ToString()).Msg("could not reset signal")
						}
					}
					u.Send(index, api.NewMessage(formatAction(action, err, ok)), nil)
				}
			}
			return nil
		}, func() {
			reports := benchmarks.assess()
			unix := time.Now().Unix()
			for c, ds := range dts {
				for d, set := range ds {
					segments := config.segments(c, d)
					for _, cfg := range segments {
						prec, err := set.fit(cfg.Model, true)
						if err != nil {
							log.Error().Err(err).
								Str("coin", string(c)).
								Str("duration", fmt.Sprintf("%+v", d)).
								Msg("could not fit dataset")
						}
						key := model.Key{
							Coin:     c,
							Duration: d,
						}
						_, err = toFile(benchmarkModelPath, key, prec, BenchTest{
							Report: reports[key],
							Config: cfg,
						}, unix)
						if err != nil {
							log.Error().Err(err).
								Str("report", fmt.Sprintf("+%v", reports[key])).
								Str("key", key.ToString()).Msg("could not save report file")
						}
					}

				}
			}
			for c, aa := range wallet.Actions() {
				fmt.Printf("c = %+v\n", c)
				for _, a := range aa {
					fmt.Printf("a = %+v\n", a)
				}
			}

		})
	}
}

func submitTrade(index api.Index, k model.Key, s Signal, wallet *trader.ExchangeTrader, u api.User, open bool, config Config) (bool, error) {
	_, ok, _, err := s.submit(k, wallet, open, config.Position.OpenValue)
	if err != nil {
		log.Error().Str("signal", fmt.Sprintf("%+v", s)).Err(err).Msg("error creating order")
		if config.Debug {
			u.Send(index, api.ErrorMessage(encodeMessage(s)).AddLine(err.Error()), nil)
		}
	} else if ok {
		u.Send(index, api.NewMessage(formatSignal(s)), nil)
	} else if config.Debug {
		u.Send(index, api.NewMessage(encodeMessage(s)), nil)
	}
	return ok, err
}

type dataset struct {
	coin     model.Coin
	duration time.Duration
	vectors  []vector
	config   Segments
}

type datasets map[model.Coin]map[time.Duration]dataset

func (dd datasets) get(c model.Coin, d time.Duration) (dataset, bool) {
	if dt, ok := dd[c]; ok {
		if tt, ok := dt[d]; ok {
			return tt, true
		}
	}
	return dataset{}, false
}

func newDataSet(coin model.Coin, duration time.Duration, cfg Segments, vv []vector) dataset {
	return dataset{
		coin:     coin,
		duration: duration,
		vectors:  vv,
		config:   cfg,
	}
}

const benchmarkModelPath = "file-storage/ml/models"
const trainDataSetPath = "file-storage/ml/datasets"
const predictDataSetPath = "file-storage/ml/tmp"

func (s dataset) predict(cfg Model) model.Type {
	fn, err := s.toFeatureFile(predictDataSetPath, fmt.Sprintf("forest_%s", "tmp_predict"), true)
	if err != nil {
		log.Error().Err(err).Msg("could not create dataset file")
		return model.NoType
	}

	predictions, err := ml.RandomForestPredict(fn, cfg.ModelSize, cfg.Features, false)
	if err != nil {
		log.Error().Err(err).Msg("could not train with isolation forest")
		return model.NoType
	}
	_, a := predictions.Size()
	lastPrediction := predictions.RowString(a - 1)
	return model.TypeFromString(lastPrediction)
}

func (s dataset) fit(cfg Model, debug bool) (float64, error) {
	hash := "tmp_fit"
	if debug {
		hash = time.Now().Format(time.RFC3339)
	}
	fn, err := s.toFeatureFile(trainDataSetPath, fmt.Sprintf("forest_%s", hash), false)
	if err != nil {
		log.Error().Err(err).Msg("could not create dataset file")
		return 0.0, err
	}

	_, _, prec, err := ml.RandomForestTrain(fn, cfg.ModelSize, cfg.Features, debug)
	if err != nil {
		log.Error().Err(err).Msg("could not train with isolation forest")
		return 0.0, err
	}
	return prec, nil
}

func (s dataset) getDescription(postfix string) string {
	return fmt.Sprintf("%s_%s_%.2f_%s", s.coin, s.duration, s.config.Model.PrecisionThreshold, postfix)
}

func (s dataset) toFeatureFile(parentPath string, postfix string, predict bool) (string, error) {

	fn, err := makePath(parentPath, fmt.Sprintf("%s.csv", s.getDescription(postfix)))
	if err != nil {
		return "", err
	}
	file, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	defer file.Close()

	if err != nil {
		return "", fmt.Errorf("could not open file: %w", err)
	}

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// take only the last n samples
	for _, vector := range s.vectors {
		lw := new(strings.Builder)
		for _, in := range vector.prevIn {
			lw.WriteString(fmt.Sprintf("%f,", in))
		}
		if vector.prevOut[0] == 1.0 {
			lw.WriteString(fmt.Sprintf("%s", model.Buy.String()))
		} else if vector.prevOut[2] == 1.0 {
			lw.WriteString(fmt.Sprintf("%s", model.Sell.String()))
		} else {
			lw.WriteString(fmt.Sprintf("%s", model.NoType.String()))
		}
		_, _ = writer.WriteString(lw.String() + "\n")
	}
	if predict {
		// for the last one add also the new value ...
		lastVector := s.vectors[len(s.vectors)-1]
		pw := new(strings.Builder)
		for _, in := range lastVector.newIn {
			pw.WriteString(fmt.Sprintf("%f,", in))
		}
		pw.WriteString(fmt.Sprintf("%s", model.NoType.String()))
		_, _ = writer.WriteString(pw.String() + "\n")
	}
	return fn, nil
}

func makePath(parentDir string, fileName string) (string, error) {
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		err := os.MkdirAll(parentDir, 0700) // Create your file
		if err != nil {
			return "", err
		}
	}
	fileName = fmt.Sprintf("%s/%s", parentDir, fileName)
	//file, _ := os.Create(fileName)
	//defer file.Close()
	return fileName, nil
}

// BenchTest defines the benchmark backtest outcome
type BenchTest struct {
	Report client.Report `json:"report"`
	Config Segments      `json:"config"`
}

func toFile(parentPath string, key model.Key, precision float64, report BenchTest, unix int64) (string, error) {
	fn, err := makePath(parentPath, fmt.Sprintf("%s_%.2f_%.0f_%d",
		key.ToString(),
		precision,
		report.Report.Profit,
		unix))
	if err != nil {
		return "", err
	}
	file, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	defer file.Close()

	if err != nil {
		return "", fmt.Errorf("could not open file: %w", err)
	}

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	bb, err := json.MarshalIndent(report, "", "\t")
	if err != nil {
		return "", fmt.Errorf("could not marshall value: %w", err)
	}
	_, err = writer.WriteString(string(bb))
	if err != nil {
		return "", fmt.Errorf("could not write file: %w", err)
	}
	return fn, nil
}