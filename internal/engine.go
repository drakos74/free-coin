package coin

import (
	"fmt"
	"sync/atomic"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

type Engine struct {
	source     api.Client
	processors []api.Processor
	count      map[model.Coin]int64
	lost       map[model.Coin]int64
}

func NewEngine(client api.Client) (*Engine, error) {
	return &Engine{
		source:     client,
		processors: make([]api.Processor, 0),
		count:      make(map[model.Coin]int64),
		lost:       make(map[model.Coin]int64),
	}, nil
}

func (e *Engine) AddProcessor(processor api.Processor) *Engine {
	e.processors = append(e.processors, processor)
	return e
}

func (e *Engine) Run() error {
	recall := make(chan api.Signal)
	source, err := e.source.Trades(recall)
	if err != nil {
		return fmt.Errorf("could not start client: %w", err)
	}

	first := e.first()

	processors := append([]api.Processor{first}, e.processors...)
	// stitch the pipeline together
	output := make(chan *model.Trade)
	for _, process := range processors {
		input := source
		go process(input, output)
		source = output
	}

	log.Info().Int("processors", len(processors)-1).Msg("engine started")

	// output processor
	for trade := range output {
		l := e.lost[trade.Coin]
		atomic.AddInt64(&l, -1)
		e.lost[trade.Coin] = l
		// signal to the source we are done processing this one
		recall <- *api.NewSignal("engine-processed").ForCoin(trade.Coin)
	}

	return nil
}

func (e *Engine) first() api.Processor {
	return func(in <-chan *model.Trade, out chan<- *model.Trade) {
		defer func() {
			close(out)
		}()
		// input processor
		for trade := range in {
			if trade == nil {
				err := e.stop()
				log.Warn().Err(err).Msg("main processor channel closed: nil trade received")
				return
			}
			// TODO : keep track of other properties of the trades
			c := e.count[trade.Coin]
			atomic.AddInt64(&c, 1)
			l := e.lost[trade.Coin]
			atomic.AddInt64(&l, 1)
			if c%10000 == 0 {
				log.Info().
					Str("trade-hash", trade.SourceID).
					Time("trade-time", trade.Time).
					Str("coin", string(trade.Coin)).
					Int64("count", c).
					Msg("processed trades")
			}
			e.count[trade.Coin] = c
			e.lost[trade.Coin] = l
			// pass over to the next processor
			out <- trade
		}
	}
}

// TODO
func (e *Engine) stop() error {
	log.Info().Msg("stopping engine")
	return nil
}
