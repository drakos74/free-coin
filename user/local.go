package user

import (
	"context"
	"fmt"

	"github.com/drakos74/free-coin/internal/api"
)

type Void struct {
}

func NewVoid() (*Void, error) {
	return &Void{}, nil
}

func (v *Void) Run(ctx context.Context) error {
	return nil
}

func (v *Void) Listen(key, prefix string) <-chan api.Command {
	return make(chan api.Command)
}

func (v *Void) Send(channel api.Index, message *api.Message, trigger *api.Trigger) int {
	// do nothing ...
	fmt.Println(fmt.Sprintf("message = %+v", message.Text))
	return -1
}
