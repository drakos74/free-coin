package ml

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/math/ml"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/drakos74/go-ex-machina/xmachina/net/ff"
	"github.com/rs/zerolog/log"
)

const (
	Name                 = "ml-network"
	mlBufferSize         = 50
	mlPrecisionThreshold = 0.61
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

	mlConfig := config.Model

	benchmarks := newBenchmarks()

	return func(u api.User, e api.Exchange) api.Processor {
		go trackUserActions(u, collector)

		datasets := make(map[time.Duration]dataset)

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
					if _, ok := datasets[d]; !ok {
						datasets[d] = newDataSet(trade.Coin, d, config.Segments[trade.Coin][d], make([]vector, 0))
					}
					newVectors := append(datasets[d].vectors, vv)
					s := len(newVectors)
					if s > mlConfig.BufferSize {
						newVectors = newVectors[s-mlConfig.BufferSize:]
					}
					datasets[d] = newDataSet(trade.Coin, d, config.Segments[trade.Coin][d], newVectors)
					// do our training here ...
					if config.Segments[trade.Coin][d].Model != "" {
						metrics.Observer.IncrementEvents(string(trade.Coin), d.String(), "train", Name)
						if len(datasets[d].vectors) >= mlConfig.BufferSize {
							metrics.Observer.IncrementEvents(string(trade.Coin), d.String(), "train_buffer", Name)
							prec, err := datasets[d].fit(mlConfig, false)
							if err != nil {
								log.Error().Err(err).Msg("could not train online")
							} else if prec > mlConfig.Threshold {
								metrics.Observer.IncrementEvents(string(trade.Coin), d.String(), "train_threshold", Name)
								t := datasets[d].predict(mlConfig)
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
										}
										// for live trading info
										u.Send(index, api.NewMessage(formatSignal(signal)).AddLine(formatReport(report)), nil)
									}
								}
							}
						}
					}
				}
				if len(signals) > 0 {
					u.Send(index, api.NewMessage(formatSignals(signals)), nil)
					//if config.Debug {
					//	var signal Signal
					//	var act bool
					//	for _, s := range signals {
					//		if signal.Type == model.NoType {
					//			signal = s
					//			act = true
					//		} else if signal.Type != s.Type || signal.Coin != s.Coin {
					//			act = false
					//		}
					//	}
					//	// TODO : get buy or sell from combination of signals
					//	signal.Duration = 0
					//	_, ok, err := signal.submit(wallet)
					//	if err != nil {
					//		log.Error().Str("signal", fmt.Sprintf("%+v", signal)).Err(err).Msg("error creating order")
					//		if config.Debug {
					//			u.Send(index, api.ErrorMessage(encodeMessage(signal)).AddLine(err.Error()), nil)
					//		}
					//	} else if ok {
					//		if config.Debug {
					//			u.Send(index, api.NewMessage(encodeMessage(signal)), nil)
					//		}
					//	}
					//}
				}
			}
			return nil
		}, func() {
			for d, set := range datasets {
				fmt.Printf("d = %+v\n", d)
				set.fit(mlConfig, true)
			}
			benchmarks.assess()
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
	//fmt.Printf("lastPrediction = %+v\n", lastPrediction)
	//fmt.Printf("predictions = %+v,%+v\n", r, a)
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
	//if prec > 0.5 {
	//	description := s.getDescription(fmt.Sprintf("%.2f", prec))
	//	modelPath, err := makePath("file-storage/ml/models", description)
	//	if err != nil {
	//		log.Error().Err(err).Str("file", description).Msg("could not save model file")
	//		return 0.0 , err
	//	}
	//	d1 := []byte(fn)
	//	err = os.WriteFile(modelPath, d1, 0644)
	//	log.Info().Err(err).Float64("precision", prec).Str("file", fn).Msg("random forest training")
	//}
	return prec, nil
}

func (s dataset) getDescription(postfix string) string {
	return fmt.Sprintf("%s_%s_%.2f_%s", s.coin, s.duration, s.config.Threshold, postfix)
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
