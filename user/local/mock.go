package local

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/drakos74/free-coin/internal/api"
)

type UserMessage struct {
	User string
	Text string
}

func NewUserMessage(s string) UserMessage {
	return UserMessage{
		Text: s,
	}
}

type MockMessage struct {
	channel api.Index
	message api.Message
	trigger *api.Trigger
}

type MockUser struct {
	*User
	messages chan MockMessage
}

func NewMockUser() *MockUser {
	user, _ := NewUser("")
	return &MockUser{
		User:     user,
		messages: make(chan MockMessage),
	}
}

func (m *MockUser) mockMessage(s string, user string, account string) {
	if account == "" {
		fmt.Println(fmt.Sprintf("s = %+v", s))
		fmt.Println(fmt.Sprintf("len(m.consumers) = %+v", len(m.consumers)))
		for _, c := range m.consumers {
			fmt.Println(fmt.Sprintf("c = %+v", c))
			c <- api.Command{
				ID:      0,
				User:    user,
				Content: s,
			}
			fmt.Println(fmt.Sprintf("account = %+v", account))
		}
	}
}

func (m *MockUser) Send(channel api.Index, message *api.Message, trigger *api.Trigger) int {

	msg := MockMessage{
		channel: channel,
		message: *message,
		trigger: trigger,
	}
	m.messages <- msg
	return 0
}

type Test struct {
	Count     int
	Assertion func(i int, message MockMessage) error
}

func NewAssertion(i int, f func(i int, message MockMessage) error) Test {
	return Test{
		Count:     i,
		Assertion: f,
	}
}

func Void() func(i int, message MockMessage) error {
	return func(i int, message MockMessage) error {
		return nil
	}
}

func Contains(ss ...string) func(i int, message MockMessage) error {
	return func(i int, message MockMessage) error {
		s := ss[i-1]
		if strings.Contains(message.message.Text, s) {
			return fmt.Errorf("substr '%s' not found in '%s'", s, message.message.Text)
		}
		return nil
	}
}

func (m *MockUser) Assert(t *testing.T, message UserMessage, test Test) {
	// give it some time for the subscription to be confirmed
	time.Sleep(1 * time.Second)

	go m.mockMessage(message.Text, message.User, "")

	c := new(sync.WaitGroup)
	c.Add(test.Count)
	go func() {
		var i int
		for msg := range m.messages {
			i++
			err := test.Assertion(i, msg)
			assert.NoError(t, err)
			c.Done()
		}
	}()

	c.Wait()
}
