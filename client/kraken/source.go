package kraken

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"time"

	krakenapi "github.com/beldur/kraken-go-api-client"
	"github.com/drakos74/free-coin/client/kraken/model"
	"github.com/drakos74/free-coin/internal/api"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	cointime "github.com/drakos74/free-coin/internal/time"
	"github.com/rs/zerolog/log"
)

const (
	Name api.ExchangeName = "kraken"
)

// Source defines a generic trade source.
// This interface allows to abstract the remote exchange logic.
type Source interface {
	io.Closer
	Trades(coin coinmodel.Coin, since int64) (*coinmodel.TradeBatch, error)
}

// RemoteSource defines a remote api for interaction with kraken exchange.
type RemoteSource struct {
	*baseSource
	Interval time.Duration
	public   *krakenapi.KrakenAPI
	count    int64
}

// AssetPairs retrieves the active asset pairs with their trading details from kraken.
func (r *RemoteSource) AssetPairs() (*krakenapi.AssetPairsResponse, error) {
	return r.public.AssetPairs()
}

// Trades retrieves the next trades batch from kraken.
func (r *RemoteSource) Trades(coin coinmodel.Coin, since int64) (*coinmodel.TradeBatch, error) {
	pair := r.converter.Coin.Pair(coin)
	log.Info().
		Str("pair", pair).
		Int64("since", since).
		Int64("count", r.count).
		Time("since-time", cointime.FromNano(since)).
		Msg("calling remote")
	// TODO : avoid the duplicate iteration on the trades
	response, err := r.public.Trades(pair, since)

	r.count += int64(len(response.Trades))
	// storing the response for the tests ...
	//rr, err := json.Marshal(response)
	//err = ioutil.WriteFile(fmt.Sprintf("testdata/response-trades/%s/%d.json", string(coin), since), rr, 0644)

	if err != nil {
		return nil, fmt.Errorf("could not get trades from kraken: %w", err)
	}
	return r.transform(pair, r.Interval, response)
}

// Close closes the kraken client.
func (r *RemoteSource) Close() error {
	return nil
}

// MockSource is a mock source that emulates the RemoteSource, but serves trades from the local filesystem.
type MockSource struct {
	*baseSource
	index map[coinmodel.Coin]int
	files map[string][]string
}

func NewMockSource(folder string) *MockSource {
	files := make(map[string][]string)
	err := filepath.Walk(folder, func(filePath string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}
		if info.IsDir() {
			files[info.Name()] = make([]string, 0)
		} else {
			dir := path.Dir(filePath)
			coin := path.Base(dir)
			files[coin] = append(files[coin], filePath)
		}

		return nil
	})
	if err != nil {
		panic(err.Error())
	}
	return &MockSource{
		baseSource: newSource(),
		index:      make(map[coinmodel.Coin]int),
		files:      files,
	}
}

func (m *MockSource) Close() error {
	// nothing to do
	return nil
}

func (m *MockSource) Trades(coin coinmodel.Coin, since int64) (*coinmodel.TradeBatch, error) {
	pair := m.converter.Coin.Pair(coin)
	files := m.files[string(coin)]
	if _, ok := m.index[coin]; !ok {
		m.index[coin] = 0
	}
	if m.index[coin] < len(files) {
		file := files[m.index[coin]]

		b, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, err
		}
		response := &krakenapi.TradesResponse{}
		err = json.Unmarshal(b, response)
		if err != nil {
			return nil, err
		}

		m.index[coin] = m.index[coin] + 1
		return m.transform(pair, time.Second, response)

	} else {
		return nil, errors.New("no trades found")
	}
}

// baseSource defines all model conversions needed for kraken remote data to be processed
type baseSource struct {
	converter model.Converter
}

func newSource() *baseSource {
	return &baseSource{converter: model.NewConverter()}
}

func (r *baseSource) transform(pair string, interval time.Duration, response *krakenapi.TradesResponse) (*coinmodel.TradeBatch, error) {
	l := len(response.Trades)
	if l == 0 {
		return &coinmodel.TradeBatch{
			Trades: []coinmodel.Trade{},
			Index:  response.Last,
		}, nil
	}
	last := cointime.FromNano(response.Last)
	var live bool
	if time.Since(last) < interval {
		live = true
	}
	trades := make([]coinmodel.Trade, l)
	for i := 0; i < l; i++ {
		trades[i] = r.newTrade(pair, i == l-1, live, response.Trades[i])
	}
	return &coinmodel.TradeBatch{
		Trades: trades,
		Index:  response.Last,
	}, nil
}

// newTrade creates a new trade from the kraken trade response.
func (r *baseSource) newTrade(pair string, active bool, live bool, trade krakenapi.TradeInfo) coinmodel.Trade {
	var t coinmodel.Type
	if trade.Buy {
		t = coinmodel.Buy
	} else if trade.Sell {
		t = coinmodel.Sell
	}
	return coinmodel.Trade{
		Exchange: "kraken",
		Coin:     r.converter.Coin.Coin(pair),
		Price:    trade.PriceFloat,
		Volume:   trade.VolumeFloat,
		Time:     time.Unix(trade.Time, 0),
		Active:   active,
		Live:     live,
		Type:     t,
	}
}

// newOrder creates a new order from a kraken order description.
// TODO : find out why this fails ... e.g. orderdescription is empty
func (r *baseSource) newOrder(order krakenapi.OrderDescription) *coinmodel.Order {
	fmt.Printf("order-description-from-kraken = %+v\n", order)
	//_, err := strconv.ParseFloat(order.PrimaryPrice, 64)
	//if err != nil {
	//	log.Error().Err(err).Str("price", order.PrimaryPrice).Msg("could not read price")
	//}
	return nil
}

// newPosition creates a new position based on the kraken position response.
func (r *baseSource) newPosition(id string, response krakenapi.Position) coinmodel.Position {
	net := float64(response.Net)
	fees := response.Fee * 2
	return coinmodel.Position{
		Data: coinmodel.Data{
			ID:      id,
			TxID:    response.TransactionID,
			OrderID: response.OrderTransactionID,
		},
		MetaData: coinmodel.MetaData{
			OpenTime: time.Unix(int64(response.TradeTime), 0),
			Net:      net,
			Fees:     fees,
			Cost:     response.Cost,
		},
		Coin:         r.converter.Coin.Coin(response.Pair),
		Type:         r.converter.Type.To(response.PositionType),
		OpenPrice:    response.Cost / response.Volume,
		CurrentPrice: response.Value / response.Volume,
		Volume:       response.Volume,
	}
}
