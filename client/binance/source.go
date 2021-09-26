package binance

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/drakos74/free-coin/client/binance/model"
	"github.com/drakos74/free-coin/internal/api"
	coinmodel "github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	Name api.ExchangeName = "binance"
)

// Source defines a generic trade source.
// This interface allows to abstract the remote exchange logic.
type Source interface {
	io.Closer
	Serve(coin coinmodel.Coin, interval time.Duration, trades chan *coinmodel.Trade) (doneC, stopC chan struct{}, err error)
}

// RemoteSource defines a remote api for interaction with kraken exchange.
type RemoteSource struct {
	*baseSource
	Interval time.Duration
}

// Serve opens a socket connection to the trade source.
// TODO: add a proper error handler
func (r *RemoteSource) Serve(coin coinmodel.Coin, interval time.Duration, trades chan *coinmodel.Trade) (doneC, stopC chan struct{}, err error) {
	return binance.WsKlineServe(r.converter.Coin.Pair(coin), r.converter.Time.From(interval), r.handler(trades), nil)
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

func newMockSource(folder string) *MockSource {
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

func (m *MockSource) Serve(coin coinmodel.Coin, interval time.Duration, trades chan *coinmodel.Trade) (doneC, stopC chan struct{}, err error) {
	done := make(chan struct{})
	stop := make(chan struct{})
	return done, stop, nil
}

// baseSource defines all model conversions needed for kraken remote data to be processed
type baseSource struct {
	converter model.Converter
}

func (b *baseSource) handler(trades chan *coinmodel.Trade) func(event *binance.WsKlineEvent) {
	return func(event *binance.WsKlineEvent) {
		trade, err := model.FromKLine(event)
		if err != nil {
			log.Error().
				Err(err).
				Str("kline", fmt.Sprintf("%+v", event)).
				Msg("binance kline")
		}
		trades <- trade
	}
}

func newSource() *baseSource {
	return &baseSource{converter: model.NewConverter()}
}
