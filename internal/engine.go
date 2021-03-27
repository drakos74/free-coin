package coin

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/drakos74/free-coin/internal/api"
	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

// Engine defines a trade source engine with the required processors to execute a trade source pipeline.
type Engine struct {
	coin       model.Coin
	uuid       string
	main       func(engineUUID string, coin model.Coin, reaction chan<- api.Action) Processor
	processors []api.Processor
	stop       chan struct{}
}

// NewEngine creates a new Engine
// c is the coin for this engine
// main can be used to extract trade information from an engine, which is inherently bound to one exchange
// autoStop is an automatically stop trigger
func NewEngine(c model.Coin, main func(engineUUID string, coin model.Coin, reaction chan<- api.Action) Processor) *Engine {
	return &Engine{
		coin:       c,
		uuid:       uuid.New().String(),
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
func (engine Engine) RunWith(finished chan<- api.Action, client api.Client) (Engine, error) {

	pipelineSignal := make(chan api.Action)

	index := uuid.New().String()

	log.Info().
		Str("coin", string(engine.coin)).
		Str("index", index).
		Str("engine-uuid", engine.uuid).
		Msg("running engine")

	tradeInput, err := client.Trades(pipelineSignal, api.Query{
		Coin:  engine.coin,
		Index: index,
	})

	if err != nil {
		return engine, fmt.Errorf("could not init trade source: %w", err)
	}

	// stitch the pipeline together
	for _, process := range engine.processors {
		input := tradeInput
		output := make(chan *model.Trade)

		go process(input, output)

		tradeInput = output
	}

	go func(reaction chan<- api.Action, engine Engine) {
		main := engine.main(engine.uuid, engine.coin, reaction)
		defer func() {
			log.Info().Msg("engine closed")
			main.Gather()
			finished <- api.NewAction("engine").
				ForCoin(engine.coin).
				WithID(engine.uuid).
				Create()
		}()
		for trade := range tradeInput {
			// pull the trades from the source through the processors
			main.Process(trade)
		}
	}(pipelineSignal, engine)

	return engine, nil

}

// Close stops the Engine.
func (engine *Engine) Close() error {
	engine.stop <- struct{}{}
	return nil
}

// engine is a wrapper around an engine and highly its dependencies.
type engine struct {
	e        *Engine
	finished chan api.Action
}

// OverWatch is the main applications wrapper that orchestrates and controls engines.
type OverWatch struct {
	ExecID  int64
	engines map[string]engine
	counter *sync.WaitGroup
	client  api.Client
	user    api.User
}

// New creates a new OverWatch instance.
func New(client api.Client, user api.User) *OverWatch {
	return &OverWatch{
		ExecID:  time.Now().Unix(),
		engines: make(map[string]engine),
		counter: new(sync.WaitGroup),
		client:  client,
		user:    user,
	}
}

// Run executes starts the OverWatch instance.
func (o *OverWatch) Run(ctx context.Context) *sync.WaitGroup {
	go func() {
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
					// TODO : find the right chat id ...
					api.Reply(api.Private, o.user, api.NewMessage(fmt.Sprintf("[error]: %s", err.Error())).ReplyTo(command.ID), err)
					continue
				}
				// ...execute
				//switch processed {
				//case "start":
				//	err = o.Start(api.NewBlock(), model.Coins[strings.ToUpper(c)], Log())
				//	api.Reply(api.Private, o.user, api.NewMessage(fmt.Sprintf("[%s]", command.Content)).ReplyTo(command.ID), err)
				//case "stop":
				//	err = o.Stop(model.Coins[strings.ToUpper(c)])
				//}
				//api.Reply(api.Private, o.user, api.NewMessage(fmt.Sprintf("[%s]", command.Content)).ReplyTo(command.ID), err)
			case <-ctx.Done():
			// kill the engines one by one
			default: // main run loop
				for c, eng := range o.engines {
					select {
					case <-eng.finished:
						log.Info().Str("coin", c).Msg("engine done")
						o.counter.Done()
					default:
						// nothing to do
					}
				}
			}
		}
	}()
	return o.counter
}

// Start starts an engine for the given coin and arguments.
func (o *OverWatch) Start(c model.Coin, processor func(engineUUID string, coin model.Coin, reaction chan<- api.Action) Processor, processors ...api.Processor) error {
	exec := engine{
		e:        NewEngine(c, processor),
		finished: make(chan api.Action),
	}
	o.counter.Add(1)
	for _, proc := range processors {
		exec.e.AddProcessor(proc)
	}
	// TODO : make the key more generic to accommodate multiple clients per coin
	e, err := exec.e.RunWith(exec.finished, o.client)
	if err != nil {
		return fmt.Errorf("could not start exec for %s: %w", c, err)
	}
	o.engines[string(c)] = exec
	log.Info().Str("exec-uuid", e.uuid).Str("coin", string(c)).Msg("started exec")
	return nil
}

// Stop stops an engine for the given coin.
func (o *OverWatch) Stop(c model.Coin) error {
	// TODO : make the key more generic to accommodate multiple clients per coin
	if e, ok := o.engines[string(c)]; ok {
		err := e.e.Close()
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

// Log is a void processor.
func Log(uuid string, coin model.Coin, processed chan<- api.Action) Processor {
	return &VoidProcessor{
		processed: processed,
		coin:      coin,
		count:     make(map[model.Coin]int64),
		lock:      new(sync.Mutex),
		uuid:      uuid,
	}
}

// VoidProcessor is the void processor struct.
type VoidProcessor struct {
	processed chan<- api.Action
	coin      model.Coin
	count     map[model.Coin]int64
	lock      *sync.Mutex
	uuid      string
}

// Process for the void processor does nothing.
func (v *VoidProcessor) Process(trade *model.Trade) {
	v.lock.Lock()
	defer v.lock.Unlock()
	// TODO : keep track of other properties of the trades
	c := v.count[trade.Coin]
	atomic.AddInt64(&c, 1)
	if c%10000 == 0 {
		log.Info().
			Str("engine-uuid", v.uuid).
			Str("trade-hash", trade.SourceID).
			Time("trade-time", trade.Time).
			Str("processor-coin", string(v.coin)).
			Str("coin", string(trade.Coin)).
			Int64("count", c).
			Msg("processed trades")
	}
	v.count[trade.Coin] = c
	// signal to the source we are done processing this one
	v.processed <- api.Action{}
}

// Gather for the void processor does nothing.
func (v *VoidProcessor) Gather() {
	// TODO : print the tracked properties for this processor.
}
