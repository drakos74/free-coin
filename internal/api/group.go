package api

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

// SendMessage is a utility method for handling the send message error.
func SendMessage(user Interface, id string, msg string, triggerFunc TriggerFunc) int {
	trigger := Trigger{
		ID:   id,
		Exec: triggerFunc,
	}
	messageID, err := user.Send(msg, &trigger)
	if err != nil {
		log.Error().Str("trigger", fmt.Sprintf("%v", trigger)).Str("message", msg).Msg("could not send message")
	}
	return messageID
}
