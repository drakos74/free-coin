package model

import (
	"github.com/drakos74/free-coin/internal/metrics"
	cointime "github.com/drakos74/free-coin/internal/time"
)

const (
	ManualConfig = "config"
	Messages     = "messages"
	FieldKey     = "field"

	RegistryFilterKey     = "registry"
	RegistryFilterRefresh = "refresh"
	RegistryFilterKeep    = "keep"

	BackTestOptionKey   = "back-test"
	BackTestOptionTrue  = "true"
	BackTestOptionFalse = "false"
)

type QQ struct {
	QK
	QV
	Range   cointime.Range
	Filters []metrics.Filter
}

type QK struct {
	Target string
	Type   string
}

type QV struct {
	Data map[string]interface{}
}
