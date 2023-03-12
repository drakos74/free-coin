package kraken

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"syscall"
	"time"

	ws "github.com/aopoltorzhicky/go_kraken/websocket"
	kraken_model "github.com/drakos74/free-coin/client/kraken/model"
	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/google/uuid"
	logger "github.com/rs/zerolog/log"
)

type Socket struct {
	coins         []string
	converter     kraken_model.CoinConverter
	typeConverter kraken_model.TypeConverter
	signals       map[model.Coin]model.TradeSignal
}

func NewSocket(coins ...model.Coin) *Socket {
	converter := kraken_model.Coin()
	cc := make([]string, len(coins))
	for i := 0; i < len(coins); i++ {
		if c, ok := converter.Pair(coins[i]); ok {
			cc[i] = c.Socket
		}
	}
	return &Socket{
		coins:         cc,
		converter:     converter,
		typeConverter: kraken_model.Type(),
		signals:       make(map[model.Coin]model.TradeSignal),
	}
}

func (s *Socket) Run(process <-chan api.Signal) (chan *model.TradeSignal, error) {
	out := make(chan *model.TradeSignal)

	go func() {
		err := s.connect(out, process)
		if err != nil {
			logger.Error().Err(err).Msg("failed to connect to socket")
		}
	}()

	return out, nil
}

func (s *Socket) connect(out chan *model.TradeSignal, process <-chan api.Signal) (cErr error) {
	defer func() {
		logger.Error().Err(cErr).Msg("error during connect, reconnecting ... ")
		time.AfterFunc(5*time.Second, func() {
			s.connect(out, process)
		})
	}()

	logger.Info().Msg("connecting to kraken socket ... ")

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	kraken := ws.NewKraken(ws.ProdBaseURL)
	if err := kraken.Connect(); err != nil {
		cErr = fmt.Errorf("error connecting to web socket: %w", err)
		return
	}

	if err := kraken.SubscribeTicker(s.coins); err != nil {
		cErr = fmt.Errorf("error for ticker subscription: %w", err)
		return
	}

	if err := kraken.SubscribeSpread(s.coins); err != nil {
		cErr = fmt.Errorf("error for spread subscription: %w", err)
		return
	}

	if err := kraken.SubscribeTrades(s.coins); err != nil {
		cErr = fmt.Errorf("error for trades subscription: %w", err)
		return
	}

	spread := make(map[model.Coin]*buffer.Window)

	for {
		select {
		case <-signals:
			if err := kraken.Close(); err != nil {
				cErr = fmt.Errorf("could not close kraken socket connection: %w", err)
			}
			close(out)
			return
		case update := <-kraken.Listen():
			coin := s.converter.Coin(update.Pair)

			if _, ok := spread[coin]; !ok {
				spread[coin] = buffer.NewWindow(0, 2)
			}

			signal := s.signals[coin]
			signal.Coin = coin

			id, _ := uuid.NewUUID()
			signal.Meta = model.Meta{
				Exchange: "kraken",
				Time:     time.Now(),
				Live:     true,
				ID:       id.String(),
			}

			switch data := update.Data.(type) {
			case ws.TickerUpdate:
				continue
			case ws.Spread:
				sp, err := spreadToSpread(data)
				if err != nil {
					logger.Err(err).Str("spread", fmt.Sprintf("%+v", update)).Msg("could not parse values")
					continue
				}
				spread[coin].Push(1, sp.Bid.Price*sp.Bid.Volume, sp.Ask.Price*sp.Ask.Volume)
			case []ws.Trade:
				tick, err := tradeToTick(data, s.typeConverter)
				if err != nil {
					logger.Err(err).Str("trade", fmt.Sprintf("%+v", update)).Msg("could not parse values")
				}
				signal.Tick = tick
				if _, b, ok := spread[coin].Push(2, 0, 0); ok {
					signal.Book = model.Book{
						Count: b.Values().Stats()[0].Count(),
						Mean:  b.Values().Stats()[0].Avg() - b.Values().Stats()[1].Avg(),
						Ratio: b.Values().Stats()[0].Ratio() - b.Values().Stats()[1].Ratio(),
						Std:   b.Values().Stats()[0].StDev() - b.Values().Stats()[1].StDev(),
					}
				}
				spread[coin] = buffer.NewWindow(0, 2)
				s.signals[coin] = signal
				if signal.Tick.Active {
					f, _ := strconv.ParseFloat(signal.Meta.Time.Format("0102.1504"), 64)
					metrics.Observer.NoteLag(f, string(coin), "socket", "ticker")
					metrics.Observer.IncrementEvents(string(coin), "_", "ticker", "socket")
					logger.Info().
						Timestamp().
						//Str("meta", fmt.Sprintf("%+v", signal.Meta)).
						Str("coin", string(signal.Coin)).
						Str("tick.level", fmt.Sprintf("%+v", signal.Tick.Level)).
						Msg("trade=debug")
					out <- &signal
					<-process
				}
			default:
				fmt.Printf("data = %+v\n %s\n", data, reflect.TypeOf(data))
			}
			if coin == model.NoCoin {
				logger.Warn().
					Str("socket", "ticker").
					Str("signal", fmt.Sprintf("%+v", signal)).
					Str("pair", update.Pair).
					Msg("unknown coin")
				continue
			}
		}
	}
}

func tradeToTick(data []ws.Trade, converter kraken_model.TypeConverter) (tick model.Tick, err error) {
	trade := data[len(data)-1]
	p, err := trade.Price.Float64()
	if err != nil {
		return tick, fmt.Errorf("could not parse price: %w", err)
	}
	v, err := trade.Volume.Float64()
	if err != nil {
		return tick, fmt.Errorf("could not parse volume: %w", err)
	}
	t, err := trade.Time.Float64()
	if err != nil {
		return tick, fmt.Errorf("could not parse time: %w", err)
	}
	tick.Level = model.Level{
		Price:  p,
		Volume: v,
	}
	tick.Time = time.Unix(int64(t), 0)
	tick.Type = converter.To(trade.Side)
	tick.Active = true
	return tick, nil
}

func tickerToSpread(data ws.TickerUpdate) (spread model.Spread, err error) {
	bp, err := data.Bid.Price.Float64()
	if err != nil {
		return spread, fmt.Errorf("could not parse bid price: %w", err)
	}
	bv, err := data.Bid.Volume.Float64()
	if err != nil {
		return spread, fmt.Errorf("could not parse bid volume: %w", err)
	}
	ap, err := data.Ask.Price.Float64()
	if err != nil {
		return spread, fmt.Errorf("could not parse ask price: %w", err)
	}
	av, err := data.Ask.Volume.Float64()
	if err != nil {
		return spread, fmt.Errorf("could not parse ask volume: %w", err)
	}
	spread.Bid = model.Level{
		Price:  bp,
		Volume: bv,
	}
	spread.Ask = model.Level{
		Price:  ap,
		Volume: av,
	}
	return spread, nil
}

func spreadToSpread(data ws.Spread) (spread model.Spread, err error) {
	bp, err := data.Bid.Float64()
	if err != nil {
		return spread, fmt.Errorf("could not parse bid price: %w", err)
	}
	bv, err := data.BidVolume.Float64()
	if err != nil {
		return spread, fmt.Errorf("could not parse bid volume: %w", err)
	}
	ap, err := data.Ask.Float64()
	if err != nil {
		return spread, fmt.Errorf("could not parse ask price: %w", err)
	}
	av, err := data.AskVolume.Float64()
	if err != nil {
		return spread, fmt.Errorf("could not parse ask volume: %w", err)
	}
	spread.Bid = model.Level{
		Price:  bp,
		Volume: bv,
	}
	spread.Ask = model.Level{
		Price:  ap,
		Volume: av,
	}
	t, err := data.Time.Float64()
	if err != nil {
		logger.Err(err).Str("ticker", fmt.Sprintf("%+v", data)).Msg("could not parse time")
		return spread, fmt.Errorf("could not parse time: %w", err)
	}
	s, n := math.Modf(t)
	spread.Time = time.Unix(int64(s), int64(n*math.Pow(10, 6)))
	return spread, nil
}
