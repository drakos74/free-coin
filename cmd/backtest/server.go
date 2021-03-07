package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/emoji"

	"github.com/drakos74/free-coin/internal/algo/processor/position"

	cointime "github.com/drakos74/free-coin/internal/time"

	"github.com/drakos74/free-coin/internal/algo/processor/trade"
	"github.com/drakos74/free-coin/internal/storage"
	jsonstore "github.com/drakos74/free-coin/internal/storage/file/json"

	"github.com/drakos74/free-coin/client/kraken"

	"github.com/drakos74/free-coin/internal/api"

	"github.com/rs/zerolog/log"

	"github.com/drakos74/free-coin/cmd/backtest/coin"
	"github.com/drakos74/free-coin/cmd/backtest/model"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog"
)

const (
	port     = 6122
	basePath = "data"

	GET  = "GET"
	POST = "POST"
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

func Handle(method string, handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
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

	http.HandleFunc(fmt.Sprintf("/%s", basePath), Handle(GET, s.live))
	http.HandleFunc(fmt.Sprintf("/%s/search", basePath), Handle(POST, s.search))
	http.HandleFunc(fmt.Sprintf("/%s/tag-keys", basePath), Handle(POST, s.keys))
	http.HandleFunc(fmt.Sprintf("/%s/tag-values", basePath), Handle(POST, s.values))
	http.HandleFunc(fmt.Sprintf("/%s/annotations", basePath), Handle(POST, s.annotations))
	http.HandleFunc(fmt.Sprintf("/%s/query", basePath), Handle(POST, s.query))

	log.Warn().Str("server", "backtest").Int("port", port).Msg("starting server")
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		return fmt.Errorf("could not start storage server: %w", err)
	}
	return nil
}

func (s *Server) query(w http.ResponseWriter, r *http.Request) {
	// we should only handle one request per time,
	// in order to ease memory footprint.
	s.block.Action <- api.NewAction("query").Create()
	defer func() {
		s.block.ReAction <- api.NewAction("query").Create()
	}()

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
	log.Debug().
		Str("query", fmt.Sprintf("%+v", query)).
		Str("endpoint", "query").
		Msg("query")
	service := coin.New()
	trades, positions, messages, err := service.Run(query)

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
	log.Debug().
		Str("body", string(body)).
		Str("endpoint", "annotations").
		Msg("request body")

	var query model.AnnotationQuery
	err = json.Unmarshal(body, &query)
	if err != nil {
		s.error(w, err)
		return
	}

	registryKeyDir := storage.RegistryPath
	//registryKeyDir := coin.BacktestRegistryDir

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
				if !(trade.Coin == coinmodel.Coin(query.Annotation.Query)) {
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

	case "trade":
		registry := jsonstore.NewEventRegistry(registryKeyDir)

		// get the events from the trade processor
		predictionPairs := []trade.PredictionPair{{}}
		err := registry.Get(storage.K{
			Pair:  query.Annotation.Query,
			Label: trade.ProcessorName,
		}, &predictionPairs)

		if err != nil {
			s.error(w, err)
			return
		}

		for _, pair := range predictionPairs {
			annotations = append(annotations, model.AnnotationInstance{
				Title: fmt.Sprintf("%s %v", pair.Key, pair.Values),
				Text:  fmt.Sprintf("%.2f %d", pair.Probability, pair.Sample),
				Time:  cointime.ToMilli(pair.Time),
				Tags:  []string{pair.Type.String(), pair.Strategy.Name},
			})
		}

	case "position_open":
		registry := jsonstore.NewEventRegistry(registryKeyDir)

		// get the events from the trade processor
		orders := []coinmodel.TrackingOrder{{}}
		err := registry.Get(storage.K{
			Pair:  query.Annotation.Query,
			Label: position.OpenPositionRegistryKey,
		}, &orders)

		if err != nil {
			s.error(w, err)
			return
		}

		for _, order := range orders {
			annotations = append(annotations, model.AnnotationInstance{
				Title: fmt.Sprintf("%2.f %.2f", order.Price, order.Volume),
				Text:  fmt.Sprintf("%s %v", order.ID, order.TxIDs),
				Time:  cointime.ToMilli(order.Time),
				Tags:  []string{order.Type.String()},
			})
		}
	case "position_close":
		registry := jsonstore.NewEventRegistry(registryKeyDir)

		// get the events from the trade processor
		positions := []coinmodel.TrackedPosition{{}}
		err := registry.Get(storage.K{
			Pair:  query.Annotation.Query,
			Label: position.ClosePositionRegistryKey,
		}, &positions)

		if err != nil {
			s.error(w, err)
			return
		}

		sort.SliceStable(positions, func(i, j int) bool {
			return positions[i].Open.Before(positions[j].Open)
		})

		for _, pos := range positions {
			net, profit := pos.Position.Value()
			annotations = append(annotations, model.AnnotationInstance{
				Title:    fmt.Sprintf("%2.f (%.2f)", net, profit),
				Text:     fmt.Sprintf("%s %v", emoji.MapToSign(profit), pos.Position.ID),
				Time:     cointime.ToMilli(pos.Open),
				TimeEnd:  cointime.ToMilli(pos.Close),
				IsRegion: true,
				Tags:     []string{pos.Position.Type.String(), emoji.MapToSign(profit)},
			})
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
			Type: "string",
			Text: "price",
		},
		{
			Type: "string",
			Text: "volume",
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

	switch tag.Key {
	case "coin":
		tags := []model.Tag{
			{
				Type: "string",
				Text: "coin",
			},
		}
		b, err := json.Marshal(tags)
		if err != nil {
			s.error(w, err)
			return
		}
		s.respond(w, b)
	default:
		s.code(w, []byte(fmt.Sprintf("unknown tag: %+v", tag)), http.StatusInternalServerError)
	}
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
