package model

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
