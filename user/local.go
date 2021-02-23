package user

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/drakos74/free-coin/internal/api"
)

const dateFormat = "Jan _2 15:04:05"

type Void struct {
	private   *log.Logger
	public    *log.Logger
	consumers map[api.ConsumerKey]chan api.Command
}

func NewVoid() (*Void, error) {
	privateFile, err := os.OpenFile("cmd/test/logs/private_messages.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	publicFile, err := os.OpenFile("cmd/test/logs/public_messages.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	return &Void{
		private:   log.New(privateFile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		public:    log.New(publicFile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		consumers: make(map[api.ConsumerKey]chan api.Command),
	}, nil
}

func (v *Void) Run(ctx context.Context) error {
	return nil
}

func (v *Void) Listen(key, prefix string) <-chan api.Command {
	ch := make(chan api.Command)
	v.consumers[api.ConsumerKey{
		Key:    key,
		Prefix: prefix,
	}] = ch
	return ch
}

func (v *Void) Send(channel api.Index, message *api.Message, trigger *api.Trigger) int {
	switch channel {
	case api.Private:
		v.private.Println(fmt.Sprintf("%s | message \n%+v", message.Ref.Format(dateFormat), message.Text))
	case api.Public:
		v.public.Println(fmt.Sprintf("%s | message = \n%+v", message.Ref.Format(dateFormat), message.Text))
	}
	// if it s an executable action ... act on it
	if trigger != nil && len(trigger.Default) > 0 {
		if trigger.Key != nil {
			key := *trigger.Key
			if ch, ok := v.consumers[key]; ok {
				ch <- api.ParseCommand(1, "self", fmt.Sprintf("%s %s", key.Prefix, strings.Join(trigger.Default, " ")))
			}
		}
	}
	// do nothing ...
	return -1
}
