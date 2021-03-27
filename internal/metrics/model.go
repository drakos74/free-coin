package metrics

import cointime "github.com/drakos74/free-coin/internal/time"

type Query struct {
	PanelID       int               `json:"panelId"`
	Range         cointime.Range    `json:"range"`
	Interval      cointime.Duration `json:"interval"`
	IntervalMS    int64             `json:"intervalMs"`
	Targets       []Target          `json:"targets"`
	MaxDataPoints int               `json:"maxDataPoints"`
	AdhocFilters  []Filter
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

type AnnotationQuery struct {
	Range      cointime.Range `json:"range"`
	Annotation Annotation
}

type Annotation struct {
	Name   string `json:"name"`
	Enable bool   `json:"enable"`
	Query  string `json:"query"`
}

type AnnotationInstance struct {
	Title    string   `json:"title"`
	Text     string   `json:"text"`
	Time     int64    `json:"time"`
	IsRegion bool     `json:"isRegion"`
	TimeEnd  int64    `json:"timeEnd"`
	Tags     []string `json:"tags"`
}

type Series struct {
	Target     string      `json:"target"`
	DataPoints [][]float64 `json:"datapoints"`
}

func NewTable() Table {
	return Table{
		Columns: make([]Column, 0),
		Rows:    make([][]string, 0),
		Type:    "table",
	}
}

type Table struct {
	Columns []Column   `json:"columns"`
	Rows    [][]string `json:"rows"`
	Type    string     `json:"type"`
}

type Column struct {
	Text string `json:"text"`
	Type string `json:"type"`
}
