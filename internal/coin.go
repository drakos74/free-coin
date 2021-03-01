package coin

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

// Engine defines a trade source engine with the required processors to execute a trade source pipeline.
type Engine struct {
	coin       model.Coin
	main       Processor
	processors []api.Processor
	stop       chan struct{}
}

// NewEngine creates a new Engine
// c is the coin for this engine
// main can be used to extract trade information from an engine, which is inherently bound to one exchange
// autoStop is an automatically stop trigger
func NewEngine(c model.Coin, main Processor) *Engine {
	return &Engine{
		coin:       c,
		main:       main,
		processors: make([]api.Processor, 0),
		stop:       make(chan struct{}),
	}
}

// AddProcessor adds a processor func to the Engine.
func (engine *Engine) AddProcessor(processor api.Processor) *Engine {
	engine.processors = append(engine.processors, processor)
	return engine
}

// RunWith starts the engine with the given client.
func (engine *Engine) RunWith(block api.Block, client api.Client) (*Engine, error) {

	trades, err := client.Trades(block.ReAction, engine.coin)

	if err != nil {
		return nil, fmt.Errorf("could not init trade source: %w", err)
	}

	for _, process := range engine.processors {

		tradeSource := make(chan *model.Trade)

		go process(trades, tradeSource)

		trades = tradeSource
	}

	go func() {
		defer func() {
			log.Info().Msg("engine closed")
			engine.main.Gather()
			block.Action <- api.Action{}
		}()
		for trade := range trades {
			// pull the trades from the source through the processors
			engine.main.Process(trade)
		}
	}()

	return engine, nil

}

// Close stops the Engine.
func (engine *Engine) Close() error {
	engine.stop <- struct{}{}
	return nil
}

// OverWatch is the main applications wrapper that orchestrates and controls engines.
type OverWatch struct {
	engines map[string]*Engine
	client  api.Client
	user    api.User
}

// New creates a new OverWatch instance.
func New(client api.Client, user api.User) *OverWatch {
	return &OverWatch{
		engines: make(map[string]*Engine),
		client:  client,
		user:    user,
	}
}

// Run executes starts the OverWatch instance.
func (o *OverWatch) Run(ctx context.Context) {
	for {
		select {
		case command := <-o.user.Listen("overwatch", "?c"):
			var c string
			var action string
			_, err := command.Validate(
				api.AnyUser(),
				api.Contains("?c", "?coin"),
				api.OneOf(&c, model.KnownCoins()...),
				api.OneOf(&action, "start", "stop"))
			if err != nil {
				api.Reply(true, o.user, api.NewMessage(fmt.Sprintf("[error]: %s", err.Error())).ReplyTo(command.ID), err)
				continue
			}
			// ...execute
			switch action {
			case "start":
				err = o.Start(api.NewBlock(), model.Coins[strings.ToUpper(c)], Log())
				api.Reply(api.Private, o.user, api.NewMessage(fmt.Sprintf("[%s]", command.Content)).ReplyTo(command.ID), err)
			case "stop":
				err = o.Stop(model.Coins[strings.ToUpper(c)])
			}
			api.Reply(api.Private, o.user, api.NewMessage(fmt.Sprintf("[%s]", command.Content)).ReplyTo(command.ID), err)
		case <-ctx.Done():
			// kill the engines one by one
		}
	}

}

// Start starts an engine for the given coin and arguments.
func (o *OverWatch) Start(block api.Block, c model.Coin, processor Processor, processors ...api.Processor) error {
	engine := NewEngine(c, processor)
	for _, proc := range processors {
		engine.AddProcessor(proc)
	}
	// TODO : make the key more generic to accommodate multiple clients per coin
	e, err := engine.RunWith(block, o.client)
	if err != nil {
		return fmt.Errorf("could not start engine for %s: %w", c, err)
	}
	o.engines[string(c)] = e
	log.Info().Str("coin", string(c)).Msg("started engine")
	return nil
}

// Stop stops an engine for the given coin.
func (o *OverWatch) Stop(c model.Coin) error {
	// TODO : make the key more generic to accommodate multiple clients per coin
	if e, ok := o.engines[string(c)]; ok {
		err := e.Close()
		if err != nil {
			log.Error().Err(err).Str("coin", string(c)).Msg("could not close engine")
			return fmt.Errorf("could not close engine for %s: %w", c, err)
		}
	}
	log.Info().Str("coin", string(c)).Msg("engine closed")
	return nil
}

// Processor is the generic engine processor.
// The difference with the internal processors is that this one allows cross-engine communication
// or interactions with the OverWatch.
type Processor interface {
	Process(trade *model.Trade)
	Gather()
}

// Log is a void processsor.
func Log() Processor {
	return &VoidProcessor{
		count: make(map[model.Coin]int),
		lock:  new(sync.Mutex),
	}
}

// VoidProcessor is the void processor struct.
type VoidProcessor struct {
	count map[model.Coin]int
	lock  *sync.Mutex
}

// Process for the void processor does nothing.
func (v *VoidProcessor) Process(trade *model.Trade) {
	v.lock.Lock()
	defer v.lock.Unlock()
	// TODO : keep track of other properties of the trades
	v.count[trade.Coin]++
	if v.count[trade.Coin]%10000 == 0 {
		log.Info().
			Time("trade-time", trade.Time).
			Str("coin", string(trade.Coin)).
			Int("count", v.count[trade.Coin]).
			Msg("processed trades")
	}
}

// Gather for the void processor does nothing.
func (v *VoidProcessor) Gather() {
	// TODO : print the tracked properties for this processor.
}
