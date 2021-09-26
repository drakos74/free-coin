package model

import (
	"fmt"
	"math"
	"time"

	"github.com/rs/zerolog/log"
)

func Time() TimeConverter {
	return TimeConverter{}
}

type TimeConverter struct {
}

func (t TimeConverter) From(duration time.Duration) string {
	return fmt.Sprintf("%0.fm", math.Ceil(duration.Minutes()))
}

func (t TimeConverter) To(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		log.Error().Err(err).Msg("could not parse duration")
	}
	return d
}
