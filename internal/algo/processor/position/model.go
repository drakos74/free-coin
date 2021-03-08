package position

import (
	"fmt"
	"sync"
	"time"

	"github.com/drakos74/free-coin/internal/algo/processor"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
)

const (
	statePair  = "state"
	stateLabel = "open_positions"
)

var (
	OpenPositionRegistryKey  = fmt.Sprintf("%s-%s", ProcessorName, "open")
	ClosePositionRegistryKey = fmt.Sprintf("%s-%s", ProcessorName, "close")
	stateKey                 = storage.Key{
		Pair:  statePair,
		Label: stateLabel,
	}
)

type tpKey struct {
	coin model.Coin
	id   string
}

func key(c model.Coin, id string) tpKey {
	return tpKey{
		coin: c,
		id:   id,
	}
}

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
	book           map[model.Coin]Portfolio
	txIDs          map[model.Coin]map[string]map[string]struct{}
	initialConfigs map[model.Coin]map[time.Duration]processor.Config
	lock           *sync.RWMutex
}
