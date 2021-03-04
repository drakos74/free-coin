package local

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"

	"github.com/drakos74/free-coin/internal/api"
)

const (
	local      = "local"
	dateFormat = "Jan _2 15:04:05"
)

type User struct {
	private   *log.Logger
	public    *log.Logger
	consumers map[api.ConsumerKey]chan api.Command
	Messages  []api.Message
	lock      *sync.RWMutex
}

func NewUser(private, public string) (*User, error) {
	var privateLogger *log.Logger
	var publicLogger *log.Logger
	if private != "" {
		privateFile, err := os.OpenFile(private, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatal(err)
		}
		privateLogger = log.New(privateFile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	}

	if public != "" {
		publicFile, err := os.OpenFile("cmd/test/logs/public_messages.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatal(err)
		}
		publicLogger = log.New(publicFile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	}

	return &User{
		private:   privateLogger,
		public:    publicLogger,
		consumers: make(map[api.ConsumerKey]chan api.Command),
		Messages:  make([]api.Message, 0),
		lock:      new(sync.RWMutex),
	}, nil
}

func (v *User) Run(ctx context.Context) error {
	return nil
}

func (v *User) Listen(key, prefix string) <-chan api.Command {
	v.lock.Lock()
	defer v.lock.Unlock()
	ch := make(chan api.Command)
	v.consumers[api.ConsumerKey{
		Key:    key,
		Prefix: prefix,
	}] = ch
	return ch
}

func (v *User) Send(channel api.Index, message *api.Message, trigger *api.Trigger) int {
	v.lock.RLock()
	defer v.lock.RUnlock()
	switch channel {
	case api.Private:
		if v.private != nil {
			v.private.Println(fmt.Sprintf("%s | message \n%+v", message.Time.Format(dateFormat), message.Text))
		}
	case api.Public:
		if v.public != nil {
			v.public.Println(fmt.Sprintf("%s | message = \n%+v", message.Time.Format(dateFormat), message.Text))
		}
	}
	v.Messages = append(v.Messages, *message)
	// if it s an executable action ... act on it
	if trigger != nil && len(trigger.Default) > 0 {
		key := trigger.Key
		if ch, ok := v.consumers[key]; ok {
			ch <- api.NewCommand(rand.Int(), local, trigger.Default...)
		}
	}
	// do nothing ...
	return -1
}
