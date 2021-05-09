package coin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"time"

	"github.com/drakos74/free-coin/client/local"
	"github.com/drakos74/free-coin/cmd/backtest/model"
	coin "github.com/drakos74/free-coin/internal"
	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/algo/processor/position"
	"github.com/drakos74/free-coin/internal/algo/processor/stats"
	"github.com/drakos74/free-coin/internal/algo/processor/trade"
	"github.com/drakos74/free-coin/internal/api"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	jsonstore "github.com/drakos74/free-coin/internal/storage/file/json"
	cointime "github.com/drakos74/free-coin/internal/time"
	userlocal "github.com/drakos74/free-coin/user/local"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type Service struct {
}

func New() *Service {
	return &Service{}
}

func CleanBackTestingDir(coin string) {
	registryPath := path.Join(storage.DefaultDir, storage.RegistryDir, storage.BackTestRegistryPath, coin)
	err := os.RemoveAll(registryPath)
	if err != nil {
		log.Warn().Msg("could not remove back-testing registry directory")
	}
}

func (s *Service) Run(ctx context.Context, query model.QQ) (map[coinmodel.Coin][]coinmodel.Trade, map[coinmodel.Coin][]coinmodel.TrackedPosition, []api.Message, error) {

	//ctx := context.Background()

	user, err := userlocal.NewUser("")
	if err != nil {
		if err != nil {
			panic(err.Error())
		}
	}

	err = user.Run(context.TODO())
	if err != nil {
		panic(err.Error())
	}

	registryFilter := true
	backtestFilter := false
	for _, filter := range query.Filters {
		switch filter.Key {
		case model.RegistryFilterKey:
			if filter.Value == model.RegistryFilterKeep {
				log.Info().Msg("skip registry refresh")
				registryFilter = false
			}
		case model.BackTestOptionKey:
			if filter.Value == model.BackTestOptionTrue {
				backtestFilter = true
			}
		}
	}

	allTrades := make(map[coinmodel.Coin][]coinmodel.Trade)
	allPositions := make(map[coinmodel.Coin][]coinmodel.TrackedPosition)

	c := coinmodel.Coin(query.Target)
	log.Info().Str("target", query.Target).Msg("run query")

	query.Range.ToInt64 = cointime.ToMilli
	tradesQuery := local.NewClient(query.Range, uuid.New().String()).
		WithPersistence(func(shard string) (storage.Persistence, error) {
			return jsonstore.NewJsonBlob("trades", shard, true), nil
		}).Mock()
	// find what the range is, in order to know how many trades to reduce
	frame := query.Range.To.Sub(query.Range.From).Hours()
	// lets say for every 24 hours we reduce by 2 the trades ... this would be
	redux := int(math.Exp2(frame / 24))
	log.Info().Float64("range", frame).Int("every", redux).Msg("reducing visible trades")

	backtestConfig := make(map[coinmodel.Coin]map[time.Duration]processor.Config)
	if config, ok := query.Data[model.ManualConfig]; ok {
		cfg, err := ReadConfig(config)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("could not init config: %w", err)
		}
		backtestConfig[c] = map[time.Duration]processor.Config{
			cointime.ToMinutes(cfg.Duration): processor.Parse(cfg),
		}
		log.Info().
			Str("config", fmt.Sprintf("%+v", config)).
			Msg("loaded config from back-test")
	}

	overWatch := coin.New(tradesQuery, user)
	finished := overWatch.Run(context.Background())

	exchange := local.
		NewExchange("").
		OneOfEvery(redux)

	registry := refreshRegistry(string(c), registryFilter)
	localStore := initLocalStorage(backtestFilter)

	block := api.NewBlock()
	statsProcessor := stats.MultiStats(localStore, registry, user, backtestConfig)
	positionProcessor := position.Position(localStore, registry, exchange, user, block, backtestConfig)
	tradeProcessor := trade.Trade(registry, user, block, backtestConfig)

	engineWrapper := func(engineUUID string, coin coinmodel.Coin, reaction chan<- api.Action) coin.Processor {
		return exchange.SignalProcessed(reaction)
	}
	err = overWatch.Start(c, engineWrapper,
		statsProcessor,
		positionProcessor,
		tradeProcessor,
	)
	go func() {
		<-ctx.Done()
		err := overWatch.Stop(c)
		if err != nil {
			log.Error().Err(err).Msg("could not stop overwatch")
		}
	}()

	if err != nil {
		return s.error(fmt.Errorf("could not start engine for '%s': %w", c, err))
	}
	// this is a long running task ... lets keep the main thread occupied
	finished.Wait()
	allTrades[c] = exchange.Trades(c)
	allPositions[c] = exchange.Positions(c)

	log.Info().Int("count", len(user.Messages)).Msg("messages")
	return allTrades, allPositions, user.Messages, nil
}

func (s *Service) error(err error) (map[coinmodel.Coin][]coinmodel.Trade, map[coinmodel.Coin][]coinmodel.TrackedPosition, []api.Message, error) {
	return nil, nil, nil, err
}

func ReadConfig(config interface{}) (processor.Config, error) {
	var cfg processor.Config
	err := FromJsonMap("", config, &cfg)
	return cfg, err
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
		fallthrough
	case "":
		return json.Unmarshal(b, n)
	}
	return fmt.Errorf("could not find json loader for config: %s", name)
}

func refreshRegistry(coin string, refresh bool) storage.Registry {
	registrySubPath := storage.BackTestRegistryPath
	registryPath := path.Join(storage.DefaultDir, storage.RegistryDir, registrySubPath, coin)
	if ok, err := IsEmpty(registryPath); refresh || ok || err != nil {
		CleanBackTestingDir(coin)
		return jsonstore.NewEventRegistry(registrySubPath).WithHash(0)
	}
	return storage.NewVoidRegistry()
}

func initLocalStorage(active bool) storage.Shard {
	if active {
		return jsonstore.BlobShard(storage.BackTestInternalPath)
	}
	return storage.VoidShard(storage.InternalPath)
}

func IsEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err // Either not empty or error, suits both cases
}
