package processor

import (
	"fmt"
	"strconv"
	"time"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/buffer"
	"github.com/drakos74/free-coin/internal/metrics"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/drakos74/free-coin/internal/storage"
	"github.com/rs/zerolog/log"
)

const (
	Name = "void"
)

// NewStateKey creates a stats internal state key for the registry storage
func NewStateKey(processor string, key model.Key) storage.Key {
	return storage.Key{
		Pair:  processor,
		Label: key.ToString(),
	}
}

// Void is a pass-through processor that only propagates trades to the next one.
func Void(name string) api.Processor {
	processorName := fmt.Sprintf("%s-%s", name, Name)
	return func(in <-chan *model.TradeSignal, out chan<- *model.TradeSignal) {
		defer func() {
			log.Info().Str("processor", processorName).Msg("closing processor")
			close(out)
		}()
		for trade := range in {
			out <- trade
		}
	}
}

// Audit creates an audit message part for the user.
func Audit(name, msg string) string {
	return fmt.Sprintf("[%s] %s", name, msg)
}

// Error creates an error message part for the user.
func Error(name string, err error) string {
	return fmt.Sprintf("[%s] error: %s", name, err.Error())
}

// Process is a wrapper for a processor logic.
func Process(name string, p func(trade *model.TradeSignal) error) api.Processor {
	return ProcessWithClose(name, p, func() {})
}

// ProcessWithClose is a wrapper for a processor logic with a close execution func.
func ProcessWithClose(name string, p func(trade *model.TradeSignal) error, shutdown func()) api.Processor {
	return func(in <-chan *model.TradeSignal, out chan<- *model.TradeSignal) {
		log.Info().Str("processor", name).Msg("started processor")
		defer func() {
			log.Info().Str("processor", name).Msg("closing processor")
			close(out)
			shutdown()
		}()
		for trade := range in {
			metrics.Observer.IncrementTrades(string(trade.Coin), name, "source")
			err := p(trade)
			if err != nil {
				log.Error().Str("processor", name).Err(err).Msg("error during processing")
			}
			out <- trade
		}
	}
}

// ProcessBufferedWithClose is a wrapper for a processor logic with a close execution func and buffering logic.
func ProcessBufferedWithClose(name string, duration time.Duration, live bool, p func(trade *model.TradeSignal) error, shutdown func(), enrich ...Enrich) api.Processor {

	signalBuffer, trades := NewSignalBuffer(duration)
	if !live {
		signalBuffer.NoLive()
	}
	//signalBuffer.WithEcho()

	// buffered signal processing happens here
	go func(trades <-chan *model.TradeSignal) {
		for signal := range trades {
			coin := string(signal.Coin)
			f, _ := strconv.ParseFloat(signal.Tick.Time.Format("20060102.1504"), 64)
			metrics.Observer.NoteLag(f, coin, Name, "source")
			for _, fn := range enrich {
				signal = fn(signal)
			}
			err := p(signal)
			if err != nil {
				log.Error().Err(err).Msg("error during trade signal processing")
			}
			// TODO : highlight the data flow better
			//log.Info().
			//	Timestamp().
			//	Str("meta", fmt.Sprintf("%+v", signal.Meta)).
			//	Str("tick.level", fmt.Sprintf("%+v", signal.Tick.Level)).
			//	Str("signal", string(signal.Coin)).
			//	Msg("buffer=debug")
		}
	}(trades)

	return func(in <-chan *model.TradeSignal, out chan<- *model.TradeSignal) {
		log.Info().Str("processor", name).Msg("started processor")
		defer func() {
			log.Info().Str("processor", name).Msg("closing processor")
			signalBuffer.Close()
			close(out)
			shutdown()
		}()
		// aggregation of signals happens here
		for trade := range in {
			metrics.Observer.IncrementTrades(string(trade.Coin), name, "source")
			// TODO : highlight the data flow better
			//log.Info().
			//	Timestamp().
			//	Str("meta", fmt.Sprintf("%+v", trade.Meta)).
			//	Str("tick", fmt.Sprintf("%+v", trade.Tick.Level)).
			//	Msg("source=debug")
			signalBuffer.Push(trade)
			out <- trade
		}
	}
}

// NoProcess is a wrapper for no processor logic
func NoProcess(name string) api.Processor {
	return Process(name, func(trade *model.TradeSignal) error {
		return nil
	})
}

// Enrich defines a function that will enrich the trade
type Enrich func(trade *model.TradeSignal) *model.TradeSignal

// Deriv defines a processor that will calculate derived values for the trade
// in this case it is the derivative of first and last value
func Deriv() Enrich {

	buf := buffer.NewMultiBuffer(2)

	return func(trade *model.TradeSignal) *model.TradeSignal {
		// TODO : highlight the data flow better
		//log.Info().
		//	Timestamp().
		//	Str("tick.level", fmt.Sprintf("%+v", trade.Tick.Level)).
		//	Str("meta", fmt.Sprintf("%+v", trade.Meta)).
		//	Msg("deriv=debug")
		if _, ok := buf.Push(float64(trade.Tick.Time.Unix()), trade.Tick.Price, trade.Tick.Volume); ok {
			vv := buf.Get()
			velocity := 0.0
			momentum := 0.0
			if vv[1][0] != vv[0][0] {
				velocity = (vv[1][1]/vv[0][1] - 1) / (vv[1][0] - vv[0][0])
				momentum = ((vv[1][1]*vv[1][2])/(vv[0][1]*vv[0][2]) - 1) / (vv[1][0] - vv[0][0])
			}
			trade.Tick.Move = model.Move{
				Velocity: velocity,
				Momentum: momentum,
			}
		}
		return trade
	}
}
