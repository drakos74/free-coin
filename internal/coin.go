package coin

import (
	"fmt"
	"strings"

	"github.com/drakos74/free-coin/coinapi"

	"github.com/drakos74/free-coin/internal/algo/model"
	"github.com/rs/zerolog/log"
)

// Engine defines a trade source engine with the required processors to execute a trade source pipeline.
type Engine struct {
	coin       model.Coin
	main       Processor
	processors []model.Processor
	autoStop   model.Condition
	stop       chan struct{}
}

// NewEngine creates a new Engine
// c is the coin for this engine
// main can be used to extract trade information from an engine, which is inherently bound to one exchange
// autoStop is an automatically stop trigger
func NewEngine(c model.Coin, main Processor, autoStop model.Condition) *Engine {
	return &Engine{
		coin:       c,
		main:       main,
		processors: make([]model.Processor, 0),
		autoStop:   autoStop,
		stop:       make(chan struct{}),
	}
}

// AddProcessor adds a processor func to the Engine.
func (engine *Engine) AddProcessor(processor model.Processor) *Engine {
	engine.processors = append(engine.processors, processor)
	return engine
}

// RunWith starts the engine with the given client.
func (engine *Engine) RunWith(client coinapi.TradeClient) (*Engine, error) {

	trades, err := client.Trades(engine.stop, engine.coin, engine.autoStop)

	if err != nil {
		return nil, fmt.Errorf("could not init trade source: %w", err)
	}

	for _, process := range engine.processors {

		tradeSource := make(chan model.Trade)

		go process(trades, tradeSource)

		trades = tradeSource
	}

	go func() {
		defer func() {
			log.Info().Msg("engine closed")
			engine.main.Gather()
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
	client  coinapi.TradeClient
	user    coinapi.UserInterface
}

// New creates a new OverWatch instance.
func New(client coinapi.TradeClient, user coinapi.UserInterface) *OverWatch {
	return &OverWatch{
		engines: make(map[string]*Engine),
		client:  client,
		user:    user,
	}
}

// Run executes starts the OverWatch instance.
func (o *OverWatch) Run() {
	for command := range o.user.Listen("overwatch", "?c") {
		var c string
		var action string
		err := command.Validate(
			coinapi.Any(),
			coinapi.Contains("?c", "?coin"),
			coinapi.OneOf(&c, model.KnownCoins()...),
			coinapi.OneOf(&action, "start", "stop"))
		if err != nil {
			o.user.Reply(coinapi.NewMessage(fmt.Sprintf("[error]: %s", err.Error())).ReplyTo(command.ID), err)
			continue
		}
		// ...execute
		switch action {
		case "start":
			err = o.Start(model.Coins[strings.ToUpper(c)], Void())
			o.user.Reply(coinapi.NewMessage(fmt.Sprintf("[%s]", command.Content)).ReplyTo(command.ID), err)
		case "stop":
			err = o.Stop(model.Coins[strings.ToUpper(c)])
		}
		o.user.Reply(coinapi.NewMessage(fmt.Sprintf("[%s]", command.Content)).ReplyTo(command.ID), err)
	}
}

// Start starts an engine for the given coin and arguments.
func (o *OverWatch) Start(c model.Coin, processor Processor, processors ...model.Processor) error {
	engine := NewEngine(c, processor, model.NonStop)
	for _, proc := range processors {
		engine.AddProcessor(proc)
	}
	// TODO : make the key more generic to accommodate multiple clients per coin
	e, err := engine.RunWith(o.client)
	if err != nil {
		return fmt.Errorf("could not start engine for %s: %w", c, err)
	}
	o.engines[string(c)] = e
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
	Process(trade model.Trade)
	Gather()
}

// Void is a void processsor.
func Void() Processor {
	return VoidProcessor{}
}

// VoidProcessor is the void processor struct.
type VoidProcessor struct {
}

// Process for the void processor does nothing.
func (v VoidProcessor) Process(trade model.Trade) {
	// nothing to do
}

// Gather for the void processor does nothing.
func (v VoidProcessor) Gather() {
	// nothing to do
}
