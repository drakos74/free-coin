package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/drakos74/free-coin/cmd/backtest/coin"
	"github.com/drakos74/free-coin/cmd/backtest/model"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog"
)

const (
	port     = 6022
	basePath = "data"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

type Server struct {
}

func New() *Server {
	return &Server{}
}

func (s *Server) Run() error {
	http.HandleFunc(fmt.Sprintf("/%s", basePath), func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(200)
		default:
			w.WriteHeader(http.StatusNotImplemented)
		}
	})

	http.HandleFunc(fmt.Sprintf("/%s/search", basePath), func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
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
		default:
			w.WriteHeader(http.StatusNotImplemented)
		}
	})

	http.HandleFunc(fmt.Sprintf("/%s/tag-keys", basePath), func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
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
		default:
			w.WriteHeader(http.StatusNotImplemented)
		}
	})

	http.HandleFunc(fmt.Sprintf("/%s/tag-values", basePath), func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
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

		default:
			w.WriteHeader(http.StatusNotImplemented)
		}
	})

	http.HandleFunc(fmt.Sprintf("/%s/annotations", basePath), func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			body, err := ioutil.ReadAll(r.Body)
			if err != nil {
				s.error(w, err)
				return
			}
			fmt.Println(fmt.Sprintf("body = %+v", string(body)))
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotImplemented)
		}
	})

	http.HandleFunc(fmt.Sprintf("/%s/query", basePath), func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
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
			fmt.Println(fmt.Sprintf("query = %+v", query))
			service := coin.New()
			trades, positions, messages, err := service.Run(query)

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
					if _, ok := target.Data[model.MultiStatsConfig]; ok {
						profitSeries := make([][]float64, 0)
						coinPositions := positions[c]
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
							Target:     fmt.Sprintf("%s P&L [%+v]", target.Target, target.Data[model.MultiStatsConfig]),
							DataPoints: profitSeries,
						})
					}
				case "table":
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
							Text: "Description",
							Type: "string",
						},
						model.Column{
							Text: "Details",
							Type: "string",
						},
					)
					for _, msg := range messages {
						txt := strings.Split(msg.Text, "\n")
						l := len(txt)
						description := make([]string, 0)
						details := make([]string, 0)
						d := ""
						if l > 5 {
							details = txt[5:]
							d = details[0]
						}
						if len(details) >= 3 {
							description = details[2:]
						}
						table.Rows = append(table.Rows, []string{msg.Time.Format(time.Stamp), txt[0], d, strings.Join(description, "\n")})
					}
					tables = append(tables, table)
				}
			}

			response := make([]interface{}, 0)
			fmt.Println(fmt.Sprintf("data.len = %+v", len(data)))
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
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotImplemented)
		}
	})

	fmt.Printf(fmt.Sprintf("Starting server at port %d\n", port))
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		return fmt.Errorf("could not start storage server: %w", err)
	}
	return nil
}

func (s *Server) error(w http.ResponseWriter, err error) {
	fmt.Println(fmt.Sprintf("err = %+v", err))
	w.Write([]byte(err.Error()))
	w.WriteHeader(http.StatusInternalServerError)
}
