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
	logger    *log.Logger
	consumers map[api.ConsumerKey]chan api.Command
	Messages  []api.Message
	lock      *sync.RWMutex
}

func NewUser(l string) (*User, error) {
	var logger *log.Logger
	if l != "" {
		privateFile, err := os.OpenFile(l, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			log.Fatal(err)
		}
		logger = log.New(privateFile, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	}

	return &User{
		logger:    logger,
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
	if v.logger != nil {
		v.logger.Println(fmt.Sprintf("%s | %s | message = \n%+v", string(channel), message.Time.Format(dateFormat), message.Text))
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

func (v *User) AddUser(channel api.Index, user string, chatID int64) error {
	return nil
}
