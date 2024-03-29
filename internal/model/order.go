package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const OrderKey = "order"

// OrderType defines the Price conditions for an order/condition i.e. market Price, limit Price etc ...
type OrderType byte

const (
	// NoOrderType means the order type is missing
	NoOrderType OrderType = iota
	// Market defines a market order
	Market
	// Limit defines a limit order
	Limit
	// StopLoss defines a stop-loss order
	StopLoss
	// TakeProfit defines a take profit order
	TakeProfit
)

// Leverage defines the Leverage involved inan order / position
type Leverage byte

const (
	// NoLeverage defines an order with no leverage
	NoLeverage Leverage = iota
	// L_3 defines a margin of 3 to 1
	L_3
	// L_4 defines a margin of 4 to 1
	L_4
	// L_5 defines a margin of 5 to 1
	L_5
)

// String returns a humanly readable representation of the leverage type.
func (l Leverage) String() string {
	switch l {
	case L_3:
		return "3:1"
	case L_4:
		return "4:1"
	case L_5:
		return "5:1"
	default:
		return ""
	}
}

// Audit defines the exact fields sent to the exchange
type Audit struct {
	Volume string `json:"volume"`
	Fills  int    `json:"fills"`
}

// TrackedOrder is an order decorated with metadata information.
type TrackedOrder struct {
	Order
	Key    Key
	Size   float64   `json:"size"`
	Ticker string    `json:"ticker"`
	RefID  string    `json:"ref_id"`
	Time   time.Time `json:"time"`
	TxIDs  []string  `json:"txIds"`
	Audit  Audit     `json:"audit"`
	Reason string    `json:"reason"`
}

func (to TrackedOrder) IsClosing() bool {
	return to.RefID != ""
}

// Value returns the value of this order in respect to our portfolio.
func (to TrackedOrder) Value() float64 {
	switch to.Type {
	case Buy:
		return -1 * to.Price * to.Volume
	case Sell:
		return to.Price * to.Volume
	}
	return 0
}

// NewTrackedOrder creates a new tracked order.
func NewTrackedOrder(key Key, time time.Time, reason string, order Order) *TrackedOrder {
	return &TrackedOrder{
		Order:  order,
		Key:    key,
		Time:   time,
		Reason: reason,
	}
}

// Order defines an order
type Order struct {
	ID       string    `json:"id"`
	Coin     Coin      `json:"coin"`
	Type     Type      `json:"type"`
	OType    OrderType `json:"order_type"`
	Volume   float64   `json:"volume"`
	Leverage Leverage  `json:"leverage"`
	Price    float64   `json:"price"`
}

// FromPosition creates an order from the position details provided
func FromPosition(position Position, close bool) Order {
	order := NewOrder(position.Coin).
		WithLeverage(L_5).
		WithVolume(position.Volume)
	if close {
		order.WithType(position.Type.Inv())
	} else {
		order.WithType(position.Type)
	}
	return order.Create()
}

// NewOrder creates a new order for the given coin.
func NewOrder(coin Coin) *Order {
	return &Order{
		ID:   uuid.New().String(),
		Coin: coin,
	}
}

// WithType defines the type of the order.
func (o *Order) WithType(t Type) *Order {
	o.mustBeEmpty(int(o.Type))
	o.Type = t
	return o
}

// Sell defines an order of type sell.
func (o *Order) Sell() *Order {
	o.mustBeEmpty(int(o.Type))
	o.Type = Sell
	return o
}

// Buy defines an order of type buy.
func (o *Order) Buy() *Order {
	o.mustBeEmpty(int(o.Type))
	o.Type = Buy
	return o
}

// WithPrice defines the price for the order (if needed).
func (o *Order) WithPrice(p float64) *Order {
	o.mustBeZero(o.Price)
	o.Price = p
	return o
}

// WithVolume defines the volume for this order.
func (o *Order) WithVolume(v float64) *Order {
	o.mustBeZero(o.Volume)
	o.Volume = v
	return o
}

// Market defines an order with market order type.
func (o *Order) Market() *Order {
	o.mustBeEmpty(int(o.OType))
	o.OType = Market
	return o
}

// Limit defines an order with market order type.
func (o *Order) Limit() *Order {
	o.mustBeEmpty(int(o.OType))
	o.OType = Limit
	return o
}

// StopLoss defines an order type of stop loss kind
func (o *Order) StopLoss() *Order {
	o.mustBeEmpty(int(o.OType))
	o.OType = StopLoss
	return o
}

// TakeProfit defines a take profit order type
func (o *Order) TakeProfit() *Order {
	o.mustBeEmpty(int(o.OType))
	o.OType = TakeProfit
	return o
}

// WithLeverage defines the leverage amount for this order.
func (o *Order) WithLeverage(l Leverage) *Order {
	o.mustBeEmpty(int(o.Leverage))
	o.Leverage = l
	return o
}

// Create creates the order based on the given details
// this will also make a sanity check on the current parameters given.
func (o *Order) Create() Order {
	// panic if we have some inconsistency
	switch o.OType {
	case StopLoss:
		fallthrough
	case TakeProfit:
		o.mustNotBeZero(o.Price)
		fallthrough
	case Market:
		o.mustNotBeZero(o.Volume)
		o.mustNotBe(byte(o.Type), byte(NoType))
	default:
		panic(fmt.Sprintf("cannot create order without 'order type': %v", o.OType))
	}
	log.Debug().Str("order", fmt.Sprintf("%+v", o)).Msg("creating order")
	return *o
}

// CreateTracked creates the tracked order from the order builder.
func (o *Order) CreateTracked(key Key, time time.Time, reason string) *TrackedOrder {
	order := o.Create()
	return NewTrackedOrder(key, time, reason, order)
}

func (o *Order) mustBeEmpty(t int) {
	if t != 0 {
		panic(fmt.Sprintf("value must be empty: %v", t))
	}
}

func (o *Order) mustNotBeZero(t float64) {
	if t <= 0 {
		panic(fmt.Sprintf("value must be larger than '0': %f", t))
	}
}

func (o *Order) mustBeZero(t float64) {
	if t != 0.0 {
		panic(fmt.Sprintf("value must be empty: %f", t))
	}
}

func (o *Order) mustNotBe(b byte, equals byte) {
	if b == equals {
		panic(fmt.Sprintf("value cannot be equal to %v: %v", equals, b))
	}
}
