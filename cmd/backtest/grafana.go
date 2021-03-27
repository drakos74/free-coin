package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/metrics"

	"github.com/drakos74/free-coin/internal/server"

	"github.com/drakos74/free-coin/client/kraken"
	"github.com/drakos74/free-coin/cmd/backtest/coin"
	"github.com/drakos74/free-coin/cmd/backtest/model"
	"github.com/drakos74/free-coin/internal/algo/processor/position"
	"github.com/drakos74/free-coin/internal/algo/processor/trade"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	AnnotationOpenPositions   = "position_open"
	AnnotationClosedPositions = "position_close"
	AnnotationTradePairs      = "trade"
	AnnotationTradeStrategy   = "strategy"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

func query(ctx context.Context, r *http.Request) (payload []byte, code int, err error) {

	var query metrics.Query
	_, err = server.ReadJson(r, true, &query)
	if err != nil {
		return payload, code, err
	}

	qq := model.QQ{
		Range:   query.Range,
		Filters: query.AdhocFilters,
	}
	for _, q := range query.Targets {
		if q.Type == "timeseries" {
			qq.QK = model.QK{
				Target: q.Target,
				Type:   "timeseries",
			}
			qq.QV = model.QV{Data: make(map[string]interface{})}
		}
		for k, v := range q.Data {
			qq.QV.Data[k] = v
		}
	}

	log.Warn().
		Str("query", fmt.Sprintf("%+v", qq)).
		Str("endpoint", "query").
		Msg("query")
	service := coin.New()
	trades, positions, messages, err := service.Run(ctx, qq)

	log.Info().Msg("service")

	if err != nil {
		return payload, code, err
	}

	data := make([]metrics.Series, 0)
	tables := make([]metrics.Table, 0)
	for _, target := range query.Targets {
		switch target.Type {
		case "timeseries":
			c := coinmodel.Coin(target.Target)
			// expose a value from the trades ... if defined
			if field, ok := target.Data[model.FieldKey]; ok {
				if f, ok := field.(string); ok {
					coinTrades := trades[c]
					log.Info().
						Int("count", len(coinTrades)).
						Str("set", f).
						Str("target", target.Target).
						Msg("adding response set")
					points := model.TradesData(f, coinTrades)
					data = append(data, metrics.Series{
						Target:     fmt.Sprintf("%s %s", target.Target, field),
						DataPoints: points,
					})
				} else {
					return payload, code, err
				}
			}
			// lets add the positions if multi stats intervals are defined
			if cfg, ok := target.Data[model.ManualConfig]; ok {
				// check tha manual config
				config, err := coin.ReadConfig(cfg)
				if err != nil {
					log.Error().Err(err).Msg("error during config parsing")
					return nil, code, err
				}
				profitSeries := make([][]float64, 0)
				coinPositions := positions[c]
				log.Info().
					Int("trades", len(trades[c])).
					Int("positions", len(coinPositions)).Str("set", model.ManualConfig).
					Str("target", target.Target).
					Msg("adding response set")
				var total float64
				for _, pos := range coinPositions {
					net, _ := pos.Value()
					// TODO : this is interesting for each position , but maybe a bit too much
					//data = append(data, model.Series{
					//	Target:     fmt.Sprintf("%s %s %.2f (%d)", target.Target, pos.Position.Type.String(), profit, id),
					//	DataPoints: model.PositionData(pos),
					//})
					total += net
					profitSeries = append(profitSeries, []float64{total, model.Time(pos.Close)})
				}
				key := coinmodel.NewKey(coinmodel.Coin(target.Target), cointime.ToMinutes(config.Duration), config.Strategy.Name)
				data = append(data, metrics.Series{
					Target:     fmt.Sprintf("P&L %s", key.ToString()),
					DataPoints: profitSeries,
				})
			}
		case "table":
			if _, ok := target.Data[model.Messages]; ok {
				log.Info().
					Int("count", len(messages)).
					Str("set", model.Messages).
					Str("target", target.Target).
					Msg("adding response set")
				table := metrics.NewTable()
				table.Columns = append(table.Columns,
					metrics.Column{
						Text: "Time",
						Type: "date",
					},
					metrics.Column{
						Text: "Message",
						Type: "string",
					},
					metrics.Column{
						Text: "Model",
						Type: "string",
					},
					metrics.Column{
						Text: "Predictions",
						Type: "string",
					},
				)
				log.Info().Int("count", len(messages)).Msg("adding messages")
				for _, msg := range messages {
					txt := strings.Split(msg.Text, "\n")
					l := len(txt)
					var stats string
					var predictions string
					if l > 1 {
						stats = strings.Join(txt[1:], " ")
					}
					if l > 5 {
						predictions = txt[5]
					}

					table.Rows = append(table.Rows, []string{msg.Time.Format(time.Stamp), txt[0], stats, predictions})
				}
				tables = append(tables, table)
			}
		}
	}

	response := make([]interface{}, 0)
	for _, table := range tables {
		response = append(response, table)
	}
	for _, d := range data {
		response = append(response, d)
	}

	payload, err = json.Marshal(response)
	return
}

func annotations(_ context.Context, r *http.Request) (payload []byte, code int, err error) {

	var query metrics.AnnotationQuery
	_, err = server.ReadJson(r, false, &query)
	if err != nil {
		return payload, code, err
	}

	if query.Annotation.Enable == false {
		return []byte("{}"), 400, nil
	}

	keys := strings.Split(query.Annotation.Query, " ")
	if len(keys) == 0 {
		return payload, 400, fmt.Errorf("query cannot be empty")
	}
	pair := keys[0]
	registryKeyDir := storage.BackTestRegistryPath
	if len(keys) > 1 {
		registryKeyDir = keys[1]
	}

	annotations := make([]metrics.AnnotationInstance, 0)
	switch query.Annotation.Name {
	case "history":
		historyClient, err := kraken.NewHistory(context.Background())
		if err != nil {
			return payload, code, err
		}

		trades, err := historyClient.Get(query.Range.From, query.Range.To)

		log.Info().Int("order", len(trades.Order)).Int("trades", len(trades.Trades)).Msg("loaded trades")

		if err != nil {
			return payload, code, err
		}
		for _, id := range trades.Order {
			trade := trades.Trades[id]
			if trade.Time.Before(query.Range.To) && trade.Time.After(query.Range.From) {
				if !(trade.Coin == coinmodel.Coin(pair)) {
					continue
				}
				if trade.RefID != "" {
					if otherTrade, ok := trades.Trades[trade.RefID]; ok {
						var tag string
						if trade.Net > 0 {
							tag = "profit"
						} else {
							tag = "loss"
						}
						// pull in the related trade
						annotations = append(annotations, metrics.AnnotationInstance{
							Text:     fmt.Sprintf("%.2f (%.2f) - %.2f", trade.Price, otherTrade.Price, trade.Volume),
							Title:    fmt.Sprintf("%s - %s ( %.2f € )", trade.Coin, trade.Type.String(), trade.Net),
							TimeEnd:  otherTrade.Time.Unix() * 1000,
							Time:     trade.Time.Unix() * 1000,
							IsRegion: true,
							Tags:     []string{tag},
						})
					} else {
						// pull in the related trade
						annotations = append(annotations, metrics.AnnotationInstance{
							Text:  fmt.Sprintf("%.2f - %.2f", trade.Price, trade.Volume),
							Title: fmt.Sprintf("%s - %s", trade.Coin, trade.Type.String()),
							Time:  trade.Time.Unix() * 1000,
						})
					}

				} else {
					// skipping position. it should be matched with a closed one
					log.Debug().Str("trade", fmt.Sprintf("%+v", trade)).Msg("open position found")
				}
			}
		}

		log.Info().
			Int("trades", len(trades.Order)).
			Int("annotations", len(annotations)).
			Msg("loaded annotations for history trades")
	case AnnotationTradePairs:
		predictionPairs, err := trade.GetPairs(registryKeyDir, pair)
		if err != nil {
			return payload, code, err
		}
		for _, pair := range predictionPairs {
			annotations = append(annotations, coin.PredictionPair(pair))
		}
	case AnnotationOpenPositions:
		orders, err := position.GetOpen(registryKeyDir, pair)
		if err != nil {
			return payload, code, err
		}
		for _, order := range orders {
			annotations = append(annotations, coin.TrackingOrder(order))
		}
	case AnnotationClosedPositions:
		positions, err := position.GetClosed(registryKeyDir, pair)
		if err != nil {
			return payload, code, err
		}
		sort.SliceStable(positions, func(i, j int) bool {
			return positions[i].Open.Before(positions[j].Open)
		})
		for _, pos := range positions {
			annotations = append(annotations, coin.TrackedPosition(pos))
		}
	case AnnotationTradeStrategy:
		strategyEvents, err := trade.StrategyEvents(registryKeyDir, pair)
		if err != nil {
			return payload, code, err
		}
		for _, event := range strategyEvents {
			if event.Sample.Valid &&
				event.Probability.Valid &&
				len(event.Probability.Values) < 4 &&
				math.Abs(event.Result.Rating) >= 4 {
				annotations = append(annotations, coin.StrategyEvent(event))
			}
		}
	}

	payload, err = json.Marshal(annotations)
	return payload, code, err
}
func keys(_ context.Context, r *http.Request) (payload []byte, code int, err error) {
	tags := []metrics.Tag{
		{
			Type: "bool",
			Text: model.RegistryFilterKey,
		},
		{
			Type: "bool",
			Text: model.BackTestOptionKey,
		},
	}
	payload, err = json.Marshal(tags)
	return payload, code, err
}

func values(_ context.Context, r *http.Request) (payload []byte, code int, err error) {
	var tag metrics.Tag
	_, err = server.ReadJson(r, false, &tag)
	if err != nil {
		return payload, code, err
	}

	values := make([]metrics.Tag, 0)
	switch tag.Key {
	case model.RegistryFilterKey:
		values = []metrics.Tag{
			{
				Type: "boolean",
				Text: model.RegistryFilterRefresh,
			},
			{
				Type: "boolean",
				Text: model.RegistryFilterKeep,
			},
		}
	case model.BackTestOptionKey:
		values = []metrics.Tag{
			{
				Type: "boolean",
				Text: model.BackTestOptionTrue,
			},
			{
				Type: "boolean",
				Text: model.BackTestOptionFalse,
			},
		}
	default:
		return []byte(fmt.Sprintf("unknown tag: %+v", tag)), http.StatusInternalServerError, err
	}
	payload, err = json.Marshal(values)
	return payload, code, err
}

func search(_ context.Context, r *http.Request) (payload []byte, code int, err error) {
	coins := make([]string, 0)
	for _, coin := range coinmodel.Coins {
		coins = append(coins, string(coin))
	}
	payload, err = json.Marshal(coins)
	return
}
