package coin

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/drakos74/free-coin/client/local"
	"github.com/drakos74/free-coin/cmd/backtest/model"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/api"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	jsonstore "github.com/drakos74/free-coin/internal/storage/file/json"
	userlocal "github.com/drakos74/free-coin/user/local"
)

type Query struct {
	output chan *coinmodel.Trade
	store  storage.Shard
	keys   []storage.Key
}

func NewQuery(coin coinmodel.Coin, queryRange model.Range) *Query {
	return &Query{
		output: make(chan *coinmodel.Trade),
		store:  nil,
		keys:   model.ToKey(coin, queryRange),
	}
}

func (q *Query) withStore(store storage.Shard) *Query {
	q.store = store
	return q
}

func (q *Query) Trades(reAction <-chan api.Action, coin coinmodel.Coin) (coinmodel.TradeSource, error) {
	go func() {
		for _, k := range q.keys {
			store, err := q.store(k.Pair)
			if err != nil {
				fmt.Println(fmt.Sprintf("could not create store = %+v", err))
			}
			var batch []coinmodel.Trade
			err = store.Load(k, &batch)
			if err != nil {
				fmt.Println(fmt.Sprintf("could not load trades = %+v", err))
			} else {
				for _, trade := range batch {
					trade.Live = true
					trade.Active = true
					q.output <- &trade
					<-reAction
				}
			}
		}
		close(q.output)
	}()
	return q.output, nil
}

type Service struct {
}

func New() *Service {
	return &Service{}
}

func (s *Service) Run(query model.Query) (map[coinmodel.Coin][]coinmodel.Trade, map[coinmodel.Coin][]local.TrackedPosition, []api.Message, error) {

	//ctx := context.Background()

	user, err := userlocal.NewUser("", "")
	if err != nil {
		if err != nil {
			panic(err.Error())
		}
	}

	err = user.Run(context.TODO())
	if err != nil {
		panic(err.Error())
	}

	allTrades := make(map[coinmodel.Coin][]coinmodel.Trade)
	allPositions := make(map[coinmodel.Coin][]local.TrackedPosition)
	// TODO : group by coin so that we dont do double the work
	for _, q := range query.Targets {
		c := coinmodel.Coin(q.Target)
		// the only difference is in the coin,
		// if we already got the trades for it, dont do it again ...
		if len(allTrades[c]) > 0 {
			fmt.Println(fmt.Sprintf("skipping duplicate run for = %+v", c))
			continue
		}
		fmt.Println(fmt.Sprintf("run query for = %+v", c))

		tradesQuery := NewQuery(c, query.Range).
			withStore(jsonstore.BlobShard("trades"))
		overWatch := coin.New(tradesQuery, user)
		//go overWatch.Run(ctx)

		finished := api.NewBlock()
		exchange := local.NewExchange("", finished.ReAction)

		block := api.NewBlock()
		multiStatsConfig := make([]processor.MultiStatsConfig, 0)
		if statsConfig, ok := q.Data[processor.StatsProcessorName]; ok {
			config, err := FromJsonMap(statsConfig)
			if err != nil {
				return s.error(fmt.Errorf("could not parse paylaod for %s: %w", processor.StatsProcessorName, err))
			}
			multiStatsConfig = append(multiStatsConfig, config)
		}

		statsProcessor := processor.MultiStats(exchange, user, multiStatsConfig...)
		positionProcessor := processor.Position(exchange, user, block, true)
		tradeProcessor := processor.Trade(exchange, user, block)

		err := overWatch.Start(finished, c, exchange,
			statsProcessor,
			positionProcessor,
			tradeProcessor,
		)

		if err != nil {
			return s.error(fmt.Errorf("could not start engine for '%s': %w", c, err))
		}
		// this is a long running task ... lets keep the main thread occupied
		<-finished.Action
		allTrades[c] = exchange.Trades(c)
		allPositions[c] = exchange.Positions(c)
	}

	return allTrades, allPositions, user.Messages, nil
}

func (s *Service) error(err error) (map[coinmodel.Coin][]coinmodel.Trade, map[coinmodel.Coin][]local.TrackedPosition, []api.Message, error) {
	return nil, nil, nil, err
}

func FromJsonMap(m interface{}) (processor.MultiStatsConfig, error) {
	// try to parse as json
	config := processor.MultiStatsConfig{
		Targets: make([]processor.Target, 0),
	}
	if configValues, ok := m.(map[string]interface{}); ok {
		if duration, ok := configValues["duration"].(float64); ok {
			config.Duration = time.Duration(duration) * time.Minute
		} else {
			return config, fmt.Errorf("could not parse duration: %v", reflect.TypeOf(configValues["duration"]))
		}
		intervals := configValues["targets"]
		s := reflect.ValueOf(intervals)
		if s.Kind() != reflect.Slice {
			return config, fmt.Errorf("invalid value for config: %s", s.Type())
		}
		// Keep the distinction between nil and empty slice input
		if s.IsNil() {
			return config, fmt.Errorf("invalid config found: %s", "nill")
		}
		for i := 0; i < s.Len(); i++ {
			cfg := s.Index(i).Interface()
			target := processor.Target{}
			if cc, ok := cfg.(map[string]interface{}); ok {
				if lookBack, ok := cc["prev"].(float64); ok {
					target.LookBack = int(lookBack)
				} else {
					return config, fmt.Errorf("could not parse target prev as int: %v", reflect.TypeOf(cc["prev"]))
				}
				if lookAhead, ok := cc["next"].(float64); ok {
					target.LookAhead = int(lookAhead)
				} else {
					return config, fmt.Errorf("could not parse target next as int: %v", reflect.TypeOf(cc["next"]))
				}
			} else {
				return config, fmt.Errorf("invalid targets found: %v", reflect.ValueOf(cfg).Type())
			}
			config.Targets = append(config.Targets, target)
		}
	} else {
		return config, fmt.Errorf("invalid struct found: %v", configValues)
	}
	return config, nil
}
