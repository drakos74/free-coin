package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/drakos74/free-coin/client/kraken"
	"github.com/drakos74/free-coin/cmd/backtest/coin"
	"github.com/drakos74/free-coin/cmd/backtest/model"
	"github.com/drakos74/free-coin/internal/algo/processor/position"
	"github.com/drakos74/free-coin/internal/algo/processor/trade"
	"github.com/drakos74/free-coin/internal/api"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	port     = 6122
	basePath = "data"

	GET  = "GET"
	POST = "POST"

	AnnotationOpenPositions   = "position_open"
	AnnotationClosedPositions = "position_close"
	AnnotationTradePairs      = "trade"
	AnnotationTradeStrategy   = "strategy"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

type Server struct {
	block api.Block
}

func New() *Server {
	return &Server{
		block: api.NewBlock(),
	}
}

func (s *Server) handle(method string, handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	// we should only handle one request per time,
	// in order to ease memory footprint.
	s.block.Action <- api.NewAction("request").Create()
	defer func() {
		s.block.ReAction <- api.NewAction("request").Create()
	}()
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case method:
			handler(w, r)
		default:
			w.WriteHeader(http.StatusNotImplemented)
		}
	}
}

func (s *Server) Run() error {

	go func() {
		for action := range s.block.Action {
			log.Warn().
				Time("time", action.Time).
				Str("action", action.Name).
				Msg("started execution")
			reaction := <-s.block.ReAction
			log.Warn().
				Time("time", action.Time).
				Float64("duration", time.Since(action.Time).Seconds()).
				Str("reaction", reaction.Name).
				Msg("completed execution")
		}
	}()

	http.HandleFunc(fmt.Sprintf("/%s", basePath), s.handle(GET, s.live))
	http.HandleFunc(fmt.Sprintf("/%s/search", basePath), s.handle(POST, s.search))
	http.HandleFunc(fmt.Sprintf("/%s/tag-keys", basePath), s.handle(POST, s.keys))
	http.HandleFunc(fmt.Sprintf("/%s/tag-values", basePath), s.handle(POST, s.values))
	http.HandleFunc(fmt.Sprintf("/%s/annotations", basePath), s.handle(POST, s.annotations))
	http.HandleFunc(fmt.Sprintf("/%s/query", basePath), s.handle(POST, s.query))

	log.Warn().Str("server", "backtest").Int("port", port).Msg("starting server")
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		return fmt.Errorf("could not start storage server: %w", err)
	}
	return nil
}

func (s *Server) query(w http.ResponseWriter, r *http.Request) {

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.error(w, err)
		return
	}
	var query model.Query
	err = json.Unmarshal(body, &query)
	if err != nil {
		s.error(w, err)
		return
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
	trades, positions, messages, err := service.Run(qq)

	log.Info().Msg("service")

	if err != nil {
		s.error(w, err)
		return
	}

	data := make([]model.Series, 0)
	tables := make([]model.Table, 0)
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
					data = append(data, model.Series{
						Target:     fmt.Sprintf("%s %s", target.Target, field),
						DataPoints: points,
					})
				} else {
					s.error(w, err)
					return
				}
			}
			// lets add the positions if multi stats intervals are defined
			if _, ok := target.Data[model.ManualConfig]; ok {
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
				data = append(data, model.Series{
					// TODO : print the config details used ... or not (?)
					Target:     fmt.Sprintf("%s P&L", target.Target),
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
				table := model.NewTable()
				table.Columns = append(table.Columns,
					model.Column{
						Text: "Time",
						Type: "date",
					},
					model.Column{
						Text: "Message",
						Type: "string",
					},
					model.Column{
						Text: "Stats",
						Type: "string",
					},
					model.Column{
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

	b, err := json.Marshal(response)
	if err != nil {
		s.error(w, err)
		return
	}
	s.respond(w, b)
}

func (s *Server) annotations(w http.ResponseWriter, r *http.Request) {

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.error(w, err)
		return
	}
	log.Warn().
		Str("body", string(body)).
		Str("endpoint", "annotations").
		Msg("request body")

	var query model.AnnotationQuery
	err = json.Unmarshal(body, &query)
	if err != nil {
		s.error(w, err)
		return
	}

	if query.Annotation.Enable == false {
		s.respond(w, []byte("{}"))
		return
	}

	keys := strings.Split(query.Annotation.Query, " ")
	if len(keys) == 0 {
		s.error(w, fmt.Errorf("query cannot be empty"))
		return
	}
	pair := keys[0]
	registryKeyDir := coin.BackTestRegistryPath
	if len(keys) > 1 {
		registryKeyDir = keys[1]
	}

	annotations := make([]model.AnnotationInstance, 0)
	switch query.Annotation.Name {
	case "history":
		historyClient, err := kraken.NewHistory(context.Background())
		if err != nil {
			s.error(w, err)
			return
		}

		trades, err := historyClient.Get(query.Range.From, query.Range.To)

		log.Info().Int("order", len(trades.Order)).Int("trades", len(trades.Trades)).Msg("loaded trades")

		if err != nil {
			s.error(w, err)
			return
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
						annotations = append(annotations, model.AnnotationInstance{
							Text:     fmt.Sprintf("%.2f (%.2f) - %.2f", trade.Price, otherTrade.Price, trade.Volume),
							Title:    fmt.Sprintf("%s - %s ( %.2f € )", trade.Coin, trade.Type.String(), trade.Net),
							TimeEnd:  otherTrade.Time.Unix() * 1000,
							Time:     trade.Time.Unix() * 1000,
							IsRegion: true,
							Tags:     []string{tag},
						})
					} else {
						// pull in the related trade
						annotations = append(annotations, model.AnnotationInstance{
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
			s.error(w, err)
			return
		}
		for _, pair := range predictionPairs {
			annotations = append(annotations, coin.PredictionPair(pair))
		}
	case AnnotationOpenPositions:
		orders, err := position.GetOpen(registryKeyDir, pair)
		if err != nil {
			s.error(w, err)
			return
		}
		for _, order := range orders {
			annotations = append(annotations, coin.TrackingOrder(order))
		}
	case AnnotationClosedPositions:
		positions, err := position.GetClosed(registryKeyDir, pair)
		if err != nil {
			s.error(w, err)
			return
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
			s.error(w, err)
			return
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

	b, err := json.Marshal(annotations)
	if err != nil {
		s.error(w, err)
		return
	}
	s.respond(w, b)
}
func (s *Server) keys(w http.ResponseWriter, r *http.Request) {
	tags := []model.Tag{
		{
			Type: "bool",
			Text: model.RegistryFilterKey,
		},
		{
			Type: "bool",
			Text: model.BackTestOptionKey,
		},
	}
	b, err := json.Marshal(tags)
	if err != nil {
		s.error(w, err)
		return
	}
	s.respond(w, b)
}

func (s *Server) values(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.error(w, err)
		return
	}
	var tag model.Tag
	err = json.Unmarshal(body, &tag)
	if err != nil {
		s.error(w, err)
	}

	values := make([]model.Tag, 0)
	switch tag.Key {
	case model.RegistryFilterKey:
		values = []model.Tag{
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
		values = []model.Tag{
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
		s.code(w, []byte(fmt.Sprintf("unknown tag: %+v", tag)), http.StatusInternalServerError)
		return
	}
	b, err := json.Marshal(values)
	if err != nil {
		s.error(w, err)
		return
	}
	s.respond(w, b)
}

func (s *Server) live(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	coins := make([]string, 0)
	for _, coin := range coinmodel.Coins {
		coins = append(coins, string(coin))
	}
	b, err := json.Marshal(coins)
	if err != nil {
		s.error(w, err)
		return
	}
	s.respond(w, b)
}

func (s *Server) code(w http.ResponseWriter, b []byte, code int) {
	s.respond(w, b)
	w.WriteHeader(code)
}

func (s *Server) respond(w http.ResponseWriter, b []byte) {
	_, err := w.Write(b)
	if err != nil {
		log.Error().Err(err).Msg("could not write response")
	}
}

func (s *Server) error(w http.ResponseWriter, err error) {
	log.Error().Err(err).Msg("error for http request")
	s.code(w, []byte(err.Error()), http.StatusInternalServerError)
}
