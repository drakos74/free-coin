package signal

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/drakos74/free-coin/internal/model"
	"github.com/rs/zerolog/log"
)

const (
	intervalKey   = "interval"
	accumulateKey = "accumulate"
	feesKey       = "fees"
	userKey       = "user"
	totalsKey     = "totals"

	minSize = 20.0

	// Index for Key definitions
	// TODO make a proper enum
	sellOff = -1
	Open    = 0
	Close   = 1
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

func (m Message) Volume(b int) (float64, float64, error) {
	// TODO : re-enable to use the right amounts
	size := minSize
	if b > minSize {
		size = float64(b)
	}
	//vol, err := strconv.ParseFloat(m.Config.Position, 64)
	//if err != nil {
	//	return 0, fmt.Errorf("could not parse volume from '%v': %w", m.Config.Position, err)
	//}
	price, err := strconv.ParseFloat(m.Data.Price, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("could not parse price from '%v': %w", m.Data.Price, err)
	}
	return size / price, size, nil
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
	return fmt.Sprintf("%s_%s", m.Data.Ticker, m.Config.Interval)
}

func (m Message) Detail() string {
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

type Orders []Order

// for sorting predictions
func (o Orders) Len() int           { return len(o) }
func (o Orders) Less(i, j int) bool { return o[i].Message.Time().Before(o[j].Message.Time()) }
func (o Orders) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }

// QueryData represents the query data details.
type QueryData struct {
	Interval time.Duration
	Acc      bool
}

func ParseQueryData(data map[string]interface{}) QueryData {
	interval, err := parseDuration(intervalKey, data)
	if err != nil {
		log.Warn().Err(err).Msg("could not parse query data")
	}
	acc := parseBool(accumulateKey, data)

	return QueryData{
		Interval: interval,
		Acc:      acc,
	}
}

func (qd QueryData) String() string {
	props := make([]string, 0)

	if qd.Interval > 0 {
		props = append(props, qd.Interval.String())
	}

	if qd.Acc {
		props = append(props, "acc")
	}

	return strings.Join(props, ":")
}
