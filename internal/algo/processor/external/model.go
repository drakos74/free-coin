package external

type Message struct {
	Config Config `json:"config"`
	Signal Signal `json:"signal"`
	Data   Data   `json:"data"`
}

type Config struct {
	SA       string `json:"SA"`
	Interval string `json:"Interval"`
	Position string `json:"Position"`
}

type Signal struct {
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
