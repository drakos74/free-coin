package external

import (
	"fmt"
	"strconv"
	"time"

	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

type Message struct {
	Config Config `json:"config"`
	Signal Action `json:"signal"`
	Data   Data   `json:"data"`
}

// TODO : make these methods part of the decoding ...

func (m Message) Price() (float64, error) {
	return strconv.ParseFloat(m.Data.Price, 64)
}

func (m Message) Volume() (float64, error) {
	// TODO : re-enable to use the right amounts
	size := 20.0
	//vol, err := strconv.ParseFloat(m.Config.Position, 64)
	//if err != nil {
	//	return 0, fmt.Errorf("could not parse volume from '%v': %w", m.Config.Position, err)
	//}
	price, err := strconv.ParseFloat(m.Data.Price, 64)
	if err != nil {
		return 0, fmt.Errorf("could not parse price from '%v': %w", m.Data.Price, err)
	}
	return size / price, nil
}

func (m Message) Type() (model.Type, error) {
	t := model.NoType
	sell, sellErr := strconv.ParseInt(m.Signal.Sell, 10, 64)
	if sellErr != nil || sell == 0 {
		sell, sellErr = strconv.ParseInt(m.Signal.StrongSell, 10, 64)
	}
	buy, buyErr := strconv.ParseInt(m.Signal.Buy, 10, 64)
	if buyErr != nil || buy == 0 {
		buy, buyErr = strconv.ParseInt(m.Signal.StrongBuy, 10, 64)
	}
	if buyErr == nil && sellErr == nil {
		if buy == sell {
			return t, fmt.Errorf("inconsistent type parsing ( buy : %v , sell : %v )", buy, sell)
		} else if buy == 1 {
			return model.Buy, nil
		} else if sell == 1 {
			return model.Sell, nil
		}
		return t, fmt.Errorf("invalid type ( buy : %v , sell : %v )", buy, sell)
	}
	return t, fmt.Errorf("could not parse type: %+v", m)
}

func (m Message) Time() time.Time {
	t, err := time.Parse(time.RFC3339, m.Data.TimeNow)
	if err != nil {
		log.Error().Err(err).Msg("could not parse time now")
	}
	return t
}

func (m Message) Duration() time.Duration {
	d, err := time.ParseDuration(m.Config.Interval)
	if err != nil {
		log.Error().Err(err).Msg("could not parse duration")
	}
	return d
}

func (m Message) Key() string {
	return fmt.Sprintf("%s_%s_%s_%s", m.Data.Ticker, m.Config.SA, m.Config.Interval, m.Config.Mode)
}

type Config struct {
	SA       string `json:"SA"`
	Interval string `json:"Interval"`
	Position string `json:"Position"`
	Mode     string `json:"Mode"` // AUTO | MANUAL
}

type Action struct {
	Buy        string `json:"Buy"`
	StrongBuy  string `json:"StrongBuy"`
	Sell       string `json:"Sell"`
	StrongSell string `json:"StrongSell"`
}

type Data struct {
	Exchange string `json:"Exchange"`
	Ticker   string `json:"Ticker"`
	Price    string `json:"Price"`
	Volume   string `json:"Volume"`
	TimeNow  string `json:"TimeNow"`
}

type Order struct {
	Message Message            `json:"message"`
	Order   model.TrackedOrder `json:"order"`
	Errors  map[string]string  `json:"errors"`
}
