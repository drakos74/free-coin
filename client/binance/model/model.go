package model

// Converter encapsulates all conversion logic for the kraken exchange.
type Converter struct {
	Coin      CoinConverter
	Type      TypeConverter
	OrderType OrderTypeConverter
	Leverage  LeverageConverter
	Time      TimeConverter
}

// NewConverter creates a new converter.
func NewConverter() Converter {
	return Converter{
		Coin:      Coin(),
		Type:      Type(),
		OrderType: OrderType(),
		Leverage:  Leverage(),
		Time:      Time(),
	}
}
