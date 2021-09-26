package coin

import (
	"fmt"

	client "github.com/drakos74/free-coin/client/local"
	"github.com/drakos74/free-coin/internal/api"
	user "github.com/drakos74/free-coin/user/local"
)

// Strategy defines a basic strategy element that encapsulates generic api components
// it serves just as a declarative wrapper for building strategies as processors for providing to the engine
type Strategy struct {
	api.User
	api.Exchange
	api.StrategyProcessor
	name string
}

func NewStrategy(name string) *Strategy {
	u, err := user.NewUser(name)
	if err != nil {
		panic(fmt.Sprintf("could not init local user: %s", err.Error()))
	}
	return &Strategy{
		User:     u,
		Exchange: client.NewExchange("temp.log"),
		name:     name,
	}
}

func (s *Strategy) ForUser(user api.User) *Strategy {
	s.User = user
	return s
}

func (s *Strategy) ForExchange(exchange api.Exchange) *Strategy {
	s.Exchange = exchange
	return s
}

func (s *Strategy) WithProcessor(processor api.StrategyProcessor) *Strategy {
	s.StrategyProcessor = processor
	return s
}

func (s *Strategy) Apply() api.Processor {
	return s.StrategyProcessor(s.User, s.Exchange)
}
