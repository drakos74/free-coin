package ml

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

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

	return func(u api.User, e api.Exchange) api.Processor {
		go trackUserActions(u, collector)

		datasets := make(map[model.Key]dataset)

		//wallet, err := trader.SimpleTrader(string(index), json.BlobShard("trader"), make(map[model.Coin]map[time.Duration]trader.Settings), e)
		//if err != nil {
		//	log.Error().Err(err).Str("processor", Name).Msg("processor in void state")
		//	return processor.NoProcess(Name)
		//}

		return processor.ProcessWithClose(Name, func(trade *model.Trade) error {
			metrics.Observer.IncrementTrades(string(trade.Coin), Name)
			signals := make(map[time.Duration]Signal)
			if vec, ok := collector.push(trade); ok {
				for d, vv := range vec {
					metrics.Observer.IncrementEvents(string(trade.Coin), d.String(), "poly", Name)
					segmentConfig := config.Segments[trade.Coin][d]
					key := model.Key{
						Coin:     trade.Coin,
						Duration: d,
					}
					if _, ok := datasets[key]; !ok {
						datasets[key] = newDataSet(trade.Coin, d, config.Segments[trade.Coin][d], make([]vector, 0))
					}
					newVectors := append(datasets[key].vectors, vv)
					s := len(newVectors)
					if s > segmentConfig.Model.BufferSize {
						newVectors = newVectors[s-segmentConfig.Model.BufferSize:]
					}
					datasets[key] = newDataSet(trade.Coin, d, config.Segments[trade.Coin][d], newVectors)
					// do our training here ...
					if segmentConfig.Model.Features > 0 {
						metrics.Observer.IncrementEvents(string(trade.Coin), d.String(), "train", Name)
						if len(datasets[key].vectors) >= segmentConfig.Model.BufferSize {
							metrics.Observer.IncrementEvents(string(trade.Coin), d.String(), "train_buffer", Name)
							prec, err := datasets[key].fit(segmentConfig.Model, false)
							if err != nil {
								log.Error().Err(err).Msg("could not train online")
							} else if prec > segmentConfig.Model.PrecisionThreshold {
								metrics.Observer.IncrementEvents(string(trade.Coin), d.String(), "train_threshold", Name)
								t := datasets[key].predict(segmentConfig.Model)
								signal := Signal{
									Coin:      trade.Coin,
									Time:      trade.Time,
									Duration:  d,
									Price:     trade.Price,
									Type:      t,
									Precision: prec,
								}
								signals[d] = signal
								if config.Debug {
									// TODO : make the exchange call on the above type
								}
								if config.Benchmark {
									report, ok, err := benchmarks.add(trade, signal)
									if err != nil {
										log.Error().Err(err).Msg("could not submit benchmark")
									} else if ok {
										if config.Debug {
											// for benchmark during backtest
											u.Send(index, api.NewMessage(encodeMessage(signal)), nil)
										} else {
											// for live trading info
											if math.Abs(time.Now().Sub(trade.Time).Hours()) <= 3 {
												u.Send(index, api.NewMessage(formatSignal(signal)).AddLine(formatReport(report)), nil)
											}
										}
									}
								}
							}
						}
					}
				}
				if len(signals) > 0 && !config.Debug {
					u.Send(index, api.NewMessage(formatSignals(signals)), nil)
					// TODO : decide how to make a unified trading strategy for the real trading
				}
			}
			return nil
		}, func() {
			reports := benchmarks.assess()
			for k, set := range datasets {
				prec, err := set.fit(config.Segments[k.Coin][k.Duration].Model, true)
				if err != nil {
					log.Error().Err(err).
						Str("key", fmt.Sprintf("+%v", k)).
						Msg("could not fit dataset")
				}
				key := trader.Key{
					Coin:     k.Coin,
					Duration: k.Duration,
				}
				_, err = toFile(benchmarkModelPath, key, prec, BenchTest{
					Report: reports[key],
					Config: config.Segments[key.Coin][key.Duration],
				})
				if err != nil {
					log.Error().Err(err).
						Str("report", fmt.Sprintf("+%v", reports[key])).
						Str("key", key.ToString()).Msg("could not save report file")
				}
			}
		})
	}
}

type dataset struct {
	coin     model.Coin
	duration time.Duration
	vectors  []vector
	config   Segments
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
	}

	predictions, err := ml.RandomForestPredict(fn, cfg.Size, cfg.Features, false)
	if err != nil {
		log.Error().Err(err).Msg("could not train with isolation forest")
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

	_, _, prec, err := ml.RandomForestTrain(fn, cfg.Size, cfg.Features, debug)
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

func toFile(parentPath string, key trader.Key, precision float64, report BenchTest) (string, error) {
	fn, err := makePath(parentPath, fmt.Sprintf("%s_%.2f_%.0f_%d", key.ToString(), precision, report.Report.Profit, time.Now().Unix()))
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
