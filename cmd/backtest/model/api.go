package model

import (
	"time"

	cointime "github.com/drakos74/free-coin/internal/time"
)

const (
	MultiStatsConfig = "stats"
	Messages         = "messages"
	FieldKey         = "field"
)

type Query struct {
	PanelID       int               `json:"panelId"`
	Range         Range             `json:"range"`
	Interval      cointime.Duration `json:"interval"`
	IntervalMS    int64             `json:"intervalMs"`
	Targets       []Target          `json:"targets"`
	MaxDataPoints int               `json:"maxDataPoints"`
	AdhocFilters  []Filter
}

type Range struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type Target struct {
	RefID  string                 `json:"refId"`
	Target string                 `json:"target"`
	Type   string                 `json:"type"`
	Data   map[string]interface{} `json:"data"`
}

type Filter struct {
	Key      string `json:"key"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type Tag struct {
	Key  string `json:"key"`
	Type string `json:"type"`
	Text string `json:"text"`
}
