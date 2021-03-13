package position

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
)

var (
	OpenPositionRegistryKey  = fmt.Sprintf("%s-%s", ProcessorName, "open")
	ClosePositionRegistryKey = fmt.Sprintf("%s-%s", ProcessorName, "close")
)

func openKey(coin model.Coin) storage.K {
	return storage.K{
		Pair:  string(coin),
		Label: OpenPositionRegistryKey,
	}
}

func closeKey(coin model.Coin) storage.K {
	return storage.K{
		Pair:  string(coin),
		Label: ClosePositionRegistryKey,
	}
}

type TradePosition struct {
	Position model.TrackedPosition `json:"book"`
	Config   processor.Strategy    `json:"config"`
}

func (tp TradePosition) updateCID(cid string) {
	pos := tp.Position
	pos.CID = cid
	tp.Position = pos
}

type Portfolio struct {
	Positions map[string]TradePosition `json:"book"`
	Budget    float64                  `json:"budget"`
}

type tradePositions struct {
	registry       storage.Registry
	state          storage.Persistence
	book           map[model.Key]Portfolio
	txIDs          map[model.Key]map[string]struct{}
	initialConfigs map[model.Coin]map[time.Duration]processor.Config
	lock           *sync.RWMutex
}

func newPositionTracker(shard storage.Shard, registry storage.Registry, configs map[model.Coin]map[time.Duration]processor.Config) *tradePositions {
	// load the book right at start-up
	state, err := shard(ProcessorName)
	if err != nil {
		log.Error().Err(err).Msg("could not init storage")
		state = storage.NewVoidStorage()
	}
	book := make(map[model.Key]Portfolio)
	// TODO : create a new portfolio for each config key ...
	//err = state.Load(stateKey, book)
	log.Info().Err(err).Int("book", len(book)).Msg("loaded book")

	return &tradePositions{
		registry:       registry,
		state:          state,
		book:           book,
		txIDs:          make(map[model.Key]map[string]struct{}),
		initialConfigs: configs,
		lock:           new(sync.RWMutex),
	}
}

// TODO : disable user adjustments for now
//func (tp *tradePositions) updateConfig(key tpKey, profit, stopLoss float64) {
//	tp.lock.Lock()
//	defer tp.lock.Unlock()
//	tp.book[key.coin][key.id].Config.Profit.Min = profit
//	tp.book[key.coin][key.id].Config.Loss.Min = stopLoss
//}

type cKey struct {
	cid   string
	coin  model.Coin
	txids []string
}
