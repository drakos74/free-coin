package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

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

	fmt.Printf(fmt.Sprintf("Starting backtest server at port %d\n", port))
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		return fmt.Errorf("could not start storage server: %w", err)
	}
	return nil
}

func (s *Server) query(w http.ResponseWriter, r *http.Request) {

	// we should only handle one request per time,
	// in order to ease memory footprint.
	s.block.Action <- api.NewAction("query")
	defer func() {
		s.block.ReAction <- api.NewAction("query")
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
	log.Warn().
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
					net, _ := pos.Position.Value()
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
						Text: "Description",
						Type: "string",
					},
					model.Column{
						Text: "Details",
						Type: "string",
					},
				)
				log.Info().Int("count", len(messages)).Msg("adding messages")
				for _, msg := range messages {
					txt := strings.Split(msg.Text, "\n")
					l := len(txt)
					var stats string
					description := make([]string, 0)
					details := make([]string, 0)
					d := ""
					if l > 1 {
						stats = txt[1]
					}
					if l > 5 {
						details = txt[5:]
						d = details[0]
					}
					if len(details) >= 3 {
						description = details[2:]
					}
					table.Rows = append(table.Rows, []string{msg.Time.Format(time.Stamp), txt[0], stats, d, strings.Join(description, "\n")})
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
	w.Write(b)
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
	w.WriteHeader(http.StatusOK)
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
	w.Write(b)
}

func (s *Server) values(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		s.error(w, err)
		return
	}
	var tag model.Tag
	err = json.Unmarshal(body, &tag)

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
		w.Write(b)
	default:
		w.Write([]byte(fmt.Sprintf("unknown tag: %+v", tag)))
		w.WriteHeader(http.StatusInternalServerError)
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
	w.Write(b)
}

func (s *Server) error(w http.ResponseWriter, err error) {
	log.Error().Err(err).Msg("error for http request")
	w.Write([]byte(err.Error()))
	w.WriteHeader(http.StatusInternalServerError)
}

func formatJsonData(v interface{}) interface{} {

	b, err := json.Marshal(v)
	if err != nil {
		log.Warn().Err(err).Msg("could not format json data")
		return v
	}

	s := string(b)

	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	s = strings.ReplaceAll(s, ":", " ")
	s = strings.ReplaceAll(s, ",", "|")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "duration", "")
	s = strings.ReplaceAll(s, "order", "")
	s = strings.ReplaceAll(s, "targets", "")

	return s

}
