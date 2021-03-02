package coin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/drakos74/free-coin/internal/algo/processor/position"
	"github.com/drakos74/free-coin/internal/algo/processor/stats"
	"github.com/drakos74/free-coin/internal/algo/processor/trade"

	"github.com/rs/zerolog/log"

	"github.com/drakos74/free-coin/client/local"
	"github.com/drakos74/free-coin/cmd/backtest/model"
	coin "github.com/drakos74/free-coin/internal"
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
				log.Warn().
					Err(err).
					Str("key", fmt.Sprintf("%+v", k)).
					Msg("could not create store")
			}
			var batch []coinmodel.Trade
			err = store.Load(k, &batch)
			if err != nil {
				log.Warn().
					Err(err).
					Str("key", fmt.Sprintf("%+v", k)).
					Msg("could not load trades")
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
	for _, q := range query.Targets {
		c := coinmodel.Coin(q.Target)
		// the only difference is in the coin,
		// if we already got the trades for it, dont do it again ...
		// TODO : this is really not the best.. but we need to stay with it , until we have our own data source plugin
		if len(allTrades[c]) > 0 {
			log.Info().Str("target", q.Target).Msg("skipping duplicate run")
			continue
		}
		log.Info().Str("target", q.Target).Msg("run query")

		tradesQuery := NewQuery(c, query.Range).
			withStore(jsonstore.BlobShard("trades"))
		overWatch := coin.New(tradesQuery, user)
		//go overWatch.Run(ctx)

		// find what the range is, in order to know how many trades to reduce
		frame := query.Range.To.Sub(query.Range.From).Hours()
		// lets say for every 24 hours we reduce by 2 the trades ... this would be
		redux := int(math.Exp2(frame / 24))
		log.Info().Float64("range", frame).Int("every", redux).Msg("reducing visible trades")

		multiStatsConfig := make([]stats.Config, 0)
		if statsConfig, ok := q.Data[stats.ProcessorName]; ok {
			var config stats.Config
			err := FromJsonMap(stats.ProcessorName, statsConfig, &config)
			if err != nil {
				return s.error(fmt.Errorf("could not parse paylaod for %s: %w", stats.ProcessorName, err))
			}
			multiStatsConfig = append(multiStatsConfig, config)
		}
		log.Warn().
			Str("processor", stats.ProcessorName).
			Str("config", fmt.Sprintf("%+v", multiStatsConfig)).
			Msg("loaded config from back-test")

		positionsConfig := make([]position.Config, 0)
		if posConfig, ok := q.Data[position.ProcessorName]; ok {
			var config position.Config
			err := FromJsonMap(position.ProcessorName, posConfig, &config)
			if err != nil {
				return s.error(fmt.Errorf("could not parse paylaod for %s: %w", position.ProcessorName, err))
			}
			positionsConfig = append(positionsConfig, config)
		}
		log.Warn().
			Str("processor", position.ProcessorName).
			Str("config", fmt.Sprintf("%+v", positionsConfig)).
			Msg("loaded config from back-test")

		tradeConfig := make([]trade.Config, 0)
		if traderConfig, ok := q.Data[trade.ProcessorName]; ok {
			var config trade.Config
			err := FromJsonMap(trade.ProcessorName, traderConfig, &config)
			if err != nil {
				return s.error(fmt.Errorf("could not parse paylaod for %s: %w", position.ProcessorName, err))
			}
			tradeConfig = append(tradeConfig, config)
		}
		log.Warn().
			Str("processor", trade.ProcessorName).
			Str("config", fmt.Sprintf("%+v", tradeConfig)).
			Msg("loaded config from back-test")

		finished := api.NewBlock()
		exchange := local.
			NewExchange("", finished.ReAction).
			OneOfEvery(redux)

		block := api.NewBlock()
		statsProcessor := stats.MultiStats(user, multiStatsConfig...)
		positionProcessor := position.Position(exchange, user, block, true, positionsConfig...)
		tradeProcessor := trade.Trade(exchange, user, block, tradeConfig...)

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
	log.Info().Int("count", len(user.Messages)).Msg("messages")
	return allTrades, allPositions, user.Messages, nil
}

func (s *Service) error(err error) (map[coinmodel.Coin][]coinmodel.Trade, map[coinmodel.Coin][]local.TrackedPosition, []api.Message, error) {
	return nil, nil, nil, err
}

func FromJsonMap(name string, m interface{}, n interface{}) error {
	// serialise and deserialize ...
	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("could not serialise config for %s", name)
	}
	switch name {
	case stats.ProcessorName:
		fallthrough
	case position.ProcessorName:
		fallthrough
	case trade.ProcessorName:
		return json.Unmarshal(b, n)
	}
	return fmt.Errorf("could not find json loader for config: %s", name)
}
